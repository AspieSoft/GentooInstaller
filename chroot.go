package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex-re2/v2"
	"github.com/AspieSoft/goutil/bash"
	"github.com/AspieSoft/goutil/cputemp"
	"github.com/AspieSoft/goutil/fs/v2"
	"github.com/AspieSoft/goutil/v7"
)

// time in minutes
const worldMaxRuntime time.Duration = 45
const worldRuntimeBreak int = 5

type chrootVars struct {
	installDisk string
	installServer bool
	installUSB bool
	cpu cpuType
	locale localeInfo
	tarName string
	diskParts diskPartList
}


func initChroot() (chrootVars) {
	bash.RunRaw(`source /etc/profile`, "", nil)
	bash.RunRaw(`export PS1="(chroot) ${PS1}"`, "", nil)
	bash.RunRaw(`setenforce 0`, "", nil)

	chrootProgressAdd(25)

	// get vars
	buf, err := os.ReadFile("/gentoo-installer/var.json")
	if err != nil {
		panic(errors.New("(chroot, true) error: failed to read vars.json"))
	}

	chrootProgressAdd(25)

	config, err := goutil.JSON.Parse(buf)
	if err != nil {
		panic(errors.New("(chroot, true) error: failed to parse vars.json"))
	}

	chrootProgressAdd(25)

	installDisk := goutil.Conv.ToString(config["installDisk"])
	installServer = goutil.ToType[bool](config["installServer"])
	installUSB = goutil.ToType[bool](config["installUSB"])
	cpu := cpuType{
		cpu: goutil.Conv.ToString(config["cpuType"]),
		cpu2: goutil.Conv.ToString(config["cpuType2"]),
		real: goutil.Conv.ToString(config["cpuReal"]),
	}
	locale := localeInfo{
		timezone: goutil.Conv.ToString(config["timezone"]),
		continent: goutil.Conv.ToString(config["continent"]),
		locale: goutil.Conv.ToString(config["locale"]),
		keymap: goutil.Conv.ToString(config["keymap"]),
	}
	tarName := goutil.Conv.ToString(config["tarName"])
	diskParts := diskPartList{
		boot: goutil.Conv.ToString(config["disk_boot"]),
		swap: goutil.Conv.ToString(config["disk_swap"]),
		root: goutil.Conv.ToString(config["disk_root"]),
		rootB: goutil.Conv.ToString(config["disk_rootB"]),
		home: goutil.Conv.ToString(config["disk_home"]),
		games: goutil.Conv.ToString(config["disk_games"]),
		mem: uint64(goutil.Conv.ToUint(config["disk_mem"])),
	}

	chrootProgressAdd(25)

	// pretend to read the gentoo eselect news, so it stops bugging me to read it
	bash.Run([]string{`eselect`, `news`, `read`}, "", nil)

	chrootWaitToCool(false)

	return chrootVars{installDisk, installServer, installUSB, cpu, locale, tarName, diskParts}
}


func installChroot(){
	config := initChroot()

	// run chroot setup
	err := chrootInitSetup(config.locale, config.diskParts, config.tarName)
	if err != nil {
		panic(err)
	}

	includeSELinux := true
	err = chrootInstallSELinux()
	if err != nil {
		if !strings.Contains(config.tarName, "selinux") { // do not print error if selinux was preinstaled
			fmt.Println(err)
		}

		includeSELinux = false
		bash.RunRaw(`setenforce 0`, "", nil)
	}

	err = emergeWorld(includeSELinux, config.diskParts)
	if err != nil {
		panic(err)
	}
	chrootWaitToCool(true)

	if includeSELinux {
		chrootSetupSELinux(config.tarName)

		bash.Run([]string{`emerge`, `--update`, `--deep`, `--newuse`, `--quiet`, `@world`}, "/", nil, true)
		chrootWaitToCool(false)
	}else if strings.Contains(config.tarName, "selinux") { // if selinux was preinstaled
		regex.Comp(`(?m)^SELINUX=.*$`).RepFileStr("/etc/selinux/config", []byte("SELINUX=enforcing"), false)
		bash.RunRaw(`setenforce 0`, "", nil)
	}

	chrootProgressAdd(5000)

	err = chrootSetup()
	if err != nil {
		panic(err)
	}

	chrootWaitToCool(false)

	err = chrootSetupSystem(config.locale, config.tarName)
	if err != nil {
		panic(err)
	}

	chrootWaitToCool(false)

	//todo: before leaving chroot, prefetch common packages for different distro variations
	// may consider running a user defined package list

	//todo: run after exiting chroot and reentering (to run after generating file)
	// installChrootPrebuild()

	// cleanup
	bash.RunRaw(`emerge --depclean --quiet`, "", nil)
	bash.RunRaw(`setsebool -P portage_use_nfs on`, "", nil)
	// bash.RunRaw(`setenforce 1`, "", nil)
}


func chrootInitSetup(locale localeInfo, diskParts diskPartList, tarName string) error {
	// move user dirs to var
	fmt.Println("(chroot) moving and linking user directories to /var")

	if _, err := os.Readlink("/home"); err != nil {
		if _, err := bash.Run([]string{`mv`, `/home`, `/var/home`}, "/", nil); err == nil {
			bash.Run([]string{`ln`, `-sr`, `./var/home`, `home`}, "/", nil)
		}
	}

	chrootProgressAdd(100)

	if _, err := os.Readlink("/root"); err != nil {
		if _, err := bash.Run([]string{`mv`, `/root`, `/var/roothome`}, "/", nil); err == nil {
			bash.Run([]string{`ln`, `-sr`, `./var/roothome`, `root`}, "/", nil)
		}
	}

	chrootProgressAdd(100)

	if _, err := os.Readlink("/usr/share"); err != nil {
		if _, err := bash.Run([]string{`mv`, `/usr/share`, `/var/usrshare`}, "/", nil); err == nil {
			bash.Run([]string{`ln`, `-sr`, `../var/usrshare`, `share`}, "/usr", nil)
		}
	}

	chrootProgressAdd(300)

	if stat, err := os.Stat("/games"); err == nil || !stat.IsDir() {
		os.Mkdir("/games", 0644)
	}

	// mount tmpfs
	if diskParts.root != "" {
		cacheSize := "2G"
		if out, err := bash.Run([]string{`lsblk`, `-linbo`, `size`, `/dev/`+diskParts.root}, "", nil); err == nil {
			if size := goutil.Conv.ToUint(out); size != 0 {
				size /= 1000 // bytes to kilobytes
				size /= 1000 // kilobytes to megabytes

				if size > 128000 {
					cacheSize = "32G"
				}else if size > 64000 {
					cacheSize = "16G"
				}else if size > 8000 {
					cacheSize = strconv.Itoa(int(size/2250))+"G"
				}else if size < 8000 {
					cacheSize = ""
				}
			}
		}

		if cacheSize != "" {
			bash.Run([]string{`mount`, `-t`, `tmpfs`, `tmpfs`, `/tmp`, `-o`, `rw,nosuid,noatime,nodev,size=`+cacheSize+`,mode=1777`}, "", nil)
		}
	}

	// install assets/theme
	fmt.Println("(chroot) installing assets/theme...")
	os.MkdirAll("/usr/share/themes", 0644)
	os.MkdirAll("/usr/share/icons", 0644)
	os.MkdirAll("/usr/share/sounds", 0644)
	os.MkdirAll("/usr/share/backgrounds", 0644)
	bash.Run([]string{`tar`, `-xzf`, `/gentoo-installer/assets/theme/themes.tar.gz`, `-C`, `/usr/share/themes`}, "/", nil)
	bash.Run([]string{`tar`, `-xzf`, `/gentoo-installer/assets/theme/icons.tar.gz`, `-C`, `/usr/share/icons`}, "/", nil)
	bash.Run([]string{`tar`, `-xzf`, `/gentoo-installer/assets/theme/sounds.tar.gz`, `-C`, `/usr/share/sounds`}, "/", nil)
	bash.Run([]string{`tar`, `-xzf`, `/gentoo-installer/assets/theme/backgrounds.tar.gz`, `-C`, `/usr/share/backgrounds`}, "/", nil)
	os.RemoveAll("/gentoo-installer/assets/theme")

	chrootProgressAdd(500)

	// install gentoo ebuild
	fmt.Println("(chroot) installing gentoo ebuild...")
	bash.Run([]string{`emerge-webrsync`}, "/", nil)

	chrootProgressAdd(1000)

	// install gentoo mirrors
	err := chrootInstallMirrors(locale)
	if err != nil {
		fmt.Println(err)
	}

	chrootProgressAdd(2500)

	// set timezone
	err = chrootSetTimezone(locale)
	if err != nil {
		fmt.Println(err)
	}

	chrootProgressAdd(2500)

	// run emerge --sync (and retry up to 3 times to handle possible race condition)
	fmt.Println("(chroot) running emerge --sync command...")
	errList := installRetry(3, "--sync")
	if len(errList) != 0 {
		return errors.New("(chroot) error: failed to run emerge --sync command")
	}

	chrootProgressAdd(3000)

	// eselect profile
	for _, tarball := range tarballList {
		// ensure accuracy of openrc/systemd
		if strings.Contains(tarName, "openrc") && !strings.Contains(tarball, "openrc") {
			continue
		}else if strings.Contains(tarName, "systemd") && !strings.Contains(tarball, "systemd") {
			continue
		}

		tarball = string(regex.Comp(`[^\w_\-/]`).RepStrLit(bytes.ReplaceAll([]byte(tarball), []byte{'-'}, []byte{'/'}), []byte{}))
		_, err = bash.RunRaw(`eselect profile set --force "$(eselect profile list | grep [0-9]/`+tarball+`[^/] | head -n1 | sed -E 's/^\s*\[[0-9]*\]\s*//' | sed -E 's/\s*\([A-Za-z0-9_-]*\)\s*(\*\s*|)$//')"`, "/", nil)
		if err == nil {
			fmt.Println("(chroot) eselect profile:", tarball)
			break
		}
	}

	chrootProgressAdd(200)

	// pretend to read the gentoo eselect news, so it stops bugging me to read it
	bash.Run([]string{`eselect`, `news`, `read`}, "", nil)

	return nil
}

func chrootInstallMirrors(locale localeInfo) error {
	errList := install("app-portage/mirrorselect")
	if len(errList) != 0 {
		return errors.New("(chroot) error: failed to install app-portage/mirrorselect")
	}

	fmt.Println("(chroot) finding fastest mirrors in your region...")
	mirrorSize := 9
	mirrors, err := bash.Run([]string{`mirrorselect`, `-s`+strconv.Itoa(mirrorSize), `-R`, locale.continent, `-o`}, "/", nil)
	for err != nil && mirrorSize > 0 {
		mirrorSize--
		mirrors, err = bash.Run([]string{`mirrorselect`, `-s`+strconv.Itoa(mirrorSize), `-R`, locale.continent, `-o`}, "/", nil)
	}

	if err != nil {
		fmt.Println("(chroot) error: failed to find any gentoo mirrors in your region")
		fmt.Println("(chroot) trying mirrors outside your region...")

		mirrorSize = 9
		mirrors, err = bash.Run([]string{`mirrorselect`, `-s`+strconv.Itoa(mirrorSize), `-o`}, "/", nil)
		for err != nil && mirrorSize > 0 {
			mirrorSize--
			mirrors, err = bash.Run([]string{`mirrorselect`, `-s`+strconv.Itoa(mirrorSize), `-o`}, "/", nil)
		}
	}

	if err != nil {
		return errors.New("(chroot) error: failed to find any gentoo mirrors")
	}

	if i := bytes.Index(mirrors, []byte("\nGENTOO_MIRRORS=")); i != -1 {
		mirrors = mirrors[i:]
	}

	err = regex.Comp(`\r?\n?GENTOO_MIRRORS="(.|[\r\n])*?"\r?\n?`).RepFileStr("/etc/portage/make.conf", mirrors, false, 102400)
	if err != nil {
		appendToFile("/etc/portage/make.conf", mirrors)
	}

	// copy repos.conf from /usr/share/portage to /etc/portage/repos.conf
	os.MkdirAll("/etc/portage/repos.conf", 0644)
	fs.Copy("/usr/share/portage/config/repos.conf", "/etc/portage/repos.conf/gentoo.conf")

	if rsyncMirror != "" {
		regex.Comp(`(?m)^(sync-uri\s*=\s*).*$`).RepFileStr("/etc/portage/repos.conf/gentoo.conf", []byte("$1"+strings.ReplaceAll(rsyncMirror, "\"'`\r\n\t\v\a\b \\/", "")+"\n"), false)
	}

	return nil
}

func chrootSetTimezone(locale localeInfo) error {
	fmt.Println("(chroot) setting timezone and locale data...")

	err := os.WriteFile("/etc/timezone", []byte(locale.timezone), 0644)
	if err != nil {
		return errors.New("(chroot) error: failed to config timezone")
	}

	_, err = bash.Run([]string{`emerge`, `--config`, `sys-libs/timezone-data`}, "/", nil)
	if err != nil {
		return errors.New("(chroot) error: failed to config timezone-data")
	}

	err = appendToFile("/etc/locale.gen", []byte(locale.locale+" ISO-8859-1"+"\n"+locale.locale+".UTF-8 UTF-8\n"), 0644)
	if err != nil {
		return errors.New("(chroot) error: failed to config locale")
	}

	_, err = bash.RunRaw(`locale-gen`, "", nil)
	if err != nil {
		return errors.New("(chroot) error: failed to run locale-gen")
	}

	_, err = bash.RunRaw(`eselect locale set "`+locale.locale+`.utf8"`, "", nil)
	if err != nil {
		return errors.New("(chroot) error: failed to eselect locale")
	}

	// reload environment
	_, err = bash.RunRaw(`env-update && source /etc/profile && export PS1="(chroot) ${PS1}"`, "", nil)
	if err != nil {
		return errors.New("(chroot) error: failed to reload environment")
	}

	return nil
}

func emergeWorld(includeSELinux bool, diskParts diskPartList) error {
	progress := uint(250000)

	var seLinuxFlags []string
	if includeSELinux {
		seLinuxFlags = []string{`FEATURES=${FEATURES} -selinux -sesandbox`}
	}

	// quick install @system
	chrootWaitToCool(true)
	fmt.Println("(chroot) installing @system...")
	_, errSYS := bash.Run([]string{`quickpkg`, `@system`}, "/", seLinuxFlags, true, false)
	if errSYS != nil {
		fmt.Println(errors.New("\n(chroot) error: failed to install @system"))
		fmt.Println("(chroot) will try again later...")
	}else{
		chrootProgressAdd(1000)
	}

	quickPkgList := map[string]error{
		`sys-devel/gcc`: nil,
		`dev-util/cmake`: nil, //? test
		// `dev-libs/json-c`: nil,
	}

	//todo: fix error: exit 2 with 'quickpkg' command (may just be in go, or could be the seLinuxFlags)

	// install quickPkgList
	for pkg := range quickPkgList {
		chrootWaitToCool(true)
		fmt.Println("(chroot) installing "+pkg+"...")
		_, err := bash.Run([]string{`quickpkg`, pkg}, "/", seLinuxFlags, true, false)
		if err != nil {
			fmt.Println(errors.New("(chroot) error: failed to install "+pkg))
			fmt.Println("(chroot) will try again later...")
			quickPkgList[pkg] = err
		}else{
			chrootProgressAdd(1000)
		}
	}

	// install dev-lang/rust-bin
	chrootWaitToCool(true)
	fmt.Println("(chroot) installing dev-lang/rust-bin...")
	_, err := bash.Run([]string{`emerge`, `--quiet`, `dev-lang/rust-bin`}, "/", seLinuxFlags, true, false)
	if err != nil {
		return errors.New("(chroot) error: failed to install dev-lang/rust-bin")
	}

	chrootProgressAdd(1000)

	// install ccache
	if diskParts.home != "" {
		cacheSize := "2G"
		if out, err := bash.Run([]string{`lsblk`, `-linbo`, `size`, `/dev/`+diskParts.home}, "", nil); err == nil {
			if size := goutil.Conv.ToUint(out); size != 0 {
				size /= 1000 // bytes to kilobytes
				size /= 1000 // kilobytes to megabytes

				if size > 128000 {
					cacheSize = "16G"
				}else if size > 64000 {
					cacheSize = "8G"
				}else if size > 32000 {
					cacheSize = "4G"
				}else if size < 32000 {
					cacheSize = ""
				}
			}
		}

		if cacheSize != "" {
			fmt.Println("(chroot) optimizing performance with ccache...")
			if errList := install(`dev-util/ccache`); len(errList) == 0 {
				hasOpt := false
				regex.Comp(`(?m)^FEAURES="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
					hasOpt = true
					return regex.JoinBytes(`FEAURES="`, data(1), ` ccache`, `"\nCCACHE_SIZE="`, cacheSize, `"\nCCACHE_DIR="/var/cache/ccache"`)
				}, false)
				if !hasOpt {
					regex.Comp(`(?m)^ACCEPT_LICENSE=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
						hasOpt = true
						return regex.JoinBytes(data(0), '\n', `FEAURES="${FEAURES}"\nCCACHE_SIZE="`, cacheSize, `"\nCCACHE_DIR="/var/cache/ccache"`)
					}, false)
				}
				if !hasOpt {
					appendToFile("/mnt/gentoo/etc/portage/make.conf", regex.JoinBytes('\n', `FEAURES="${FEAURES}"\nCCACHE_SIZE="`, cacheSize, `"\nCCACHE_DIR="/var/cache/ccache"`, '\n'))
				}

				os.MkdirAll("/var/cache/ccache", 0644)
				os.WriteFile("/var/cache/ccache/ccache.conf", regex.JoinBytes(
					`max_size = `, cacheSize, '\n',
					`umask = 002`, '\n',
					`hash_dir = false`, '\n',
					`cache_dir_levels = 3`, '\n',
					`compression = true`, '\n',
					`compression_level = 1`, '\n',
					'\n',
					`# Preserve cache across GCC rebuilds`, '\n',
					`compiler_check = %compiler% -dumpversion`, '\n',
				), 0644)
			}
		}
	}

	chrootProgressAdd(1000)

	// run emerge -f @world
	chrootWaitToCool(true)
	fmt.Println("(chroot) running emerge -f @world...")

	// emerge --autounmask-write @world
	_, err = bash.Run([]string{`emerge`, `--update`, `--deep`, `--newuse`, `--quiet`, `--autounmask-write`, `-f`, `@world`}, "/", seLinuxFlags, true, false)

	if err != nil {
		// etc-update
		bash.Pipe("/", []string{`echo`, `-e`, `y\n`}, []string{`etc-update`, `--automode`, `-3`})

		// emerge --fetch @world
		// bash.Run([]string{`emerge`, `--update`, `--deep`, `--newuse`, `--quiet`, `-f`, `@world`}, "/", seLinuxFlags, true, false)
	}

	chrootProgressAdd(1000)

	if errSYS != nil {
		fmt.Println("(chroot) installing @system...")
		_, errSYS = bash.Run([]string{`quickpkg`, `@system`}, "/", seLinuxFlags, true, false)
		/* if errSYS != nil {
			fmt.Println(errors.New("\n(chroot) error: failed to install @system"))
			fmt.Println("(chroot) trying emerge instead of quicksync...")
			_, errSYS = bash.Run([]string{`emerge`, `--quiet`, `@system`}, "/", seLinuxFlags, true, false)
		} */
		if errSYS != nil {
			fmt.Println(errors.New("\n(chroot) error: failed to install @system"))
			// return errors.New("(chroot) error: failed to install @system")
		}
		chrootProgressAdd(1000)
	}

	// retry quickPkgList
	for pkg, err := range quickPkgList {
		if err != nil {
			fmt.Println("(chroot) installing "+pkg+"...")
			_, err = bash.Run([]string{`quickpkg`, pkg}, "/", seLinuxFlags, true, false)
			if err != nil {
				fmt.Println(errors.New("(chroot) error: failed to install "+pkg))
				fmt.Println("(chroot) trying emerge instead of quicksync...")
				_, err = bash.Run([]string{`emerge`, `--quiet`, pkg}, "/", seLinuxFlags, true, false)
			}
			if err != nil {
				return errors.New("(chroot) error: failed to install "+pkg)
			}
			chrootProgressAdd(1000)
		}
	}

	var progressStep uint
	var cProgress uint

	// run emerge @world
	chrootWaitToCool(true)
	fmt.Println("(chroot) running emerge @world...")

	//todo: fix gcc and cmake taking forever to install
	// seLinuxFlags = append(seLinuxFlags, `USE=${USE} -gcc -cmake`)

	cmd := exec.Command(`emerge`, `--update`, `--deep`, `--newuse`, `--quiet`, `@world`)
	cmd.Dir = "/"
	if includeSELinux {
		if cmd.Env == nil {
			cmd.Env = []string{}
		}
		cmd.Env = append(cmd.Env, seLinuxFlags...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	cmd.Start()

	runTime := time.Now().Add(worldMaxRuntime * time.Minute)

	sleepMode := false
	var installErr error
	go func(){
		for {
			time.Sleep(10 * time.Second)

			if !sleepMode && time.Now().UnixMilli() > runTime.UnixMilli() {
				sleepMode = true
				chrootProgressPrint(false)
				time.Sleep(5 * time.Second)
				cmd.Process.Signal(os.Kill)

				fmt.Println("CPU has been running this task for "+strconv.Itoa(int(worldMaxRuntime))+" minutes and needs a "+strconv.Itoa(worldRuntimeBreak)+" minute break!")
				fmt.Print("Process will resume in "+strconv.Itoa(worldRuntimeBreak)+" minutes...          \r")

				for i := worldRuntimeBreak; i >= 0; i-- {
					time.Sleep(1 * time.Minute)
					fmt.Print("Process will resume in "+strconv.Itoa(i)+" minutes...          \r")
				}

				fmt.Println("Resuming Process...                    ")

				runTime = time.Now().Add(worldMaxRuntime * time.Minute)

				cmd = exec.Command(`emerge`, `--resume`, `--quiet`)
				cmd.Dir = "/"
				if includeSELinux {
					if cmd.Env == nil {
						cmd.Env = []string{}
					}
					cmd.Env = append(cmd.Env, seLinuxFlags...)
				}

				stdout, err = cmd.StdoutPipe()
				if err != nil {
					installErr = err
					return
				}

				cmd.Start()
				time.Sleep(5 * time.Second)
				sleepMode = false
				chrootProgressPrint(true)
			}

			temp := cputemp.GetTemp()
			if !sleepMode && temp >= cputemp.HighTemp {
				sleepMode = true
				// chrootProgressPrint(false)
				time.Sleep(5 * time.Second)
				cmd.Process.Signal(os.Kill)

				chrootWaitToCool(true)

				cmd = exec.Command(`emerge`, `--resume`, `--quiet`)
				cmd.Dir = "/"
				if includeSELinux {
					if cmd.Env == nil {
						cmd.Env = []string{}
					}
					cmd.Env = append(cmd.Env, seLinuxFlags...)
				}

				stdout, err = cmd.StdoutPipe()
				if err != nil {
					installErr = err
					return
				}

				cmd.Start()
				time.Sleep(5 * time.Second)
				sleepMode = false
				// chrootProgressPrint(true)
			}
		}
	}()

	for {
		time.Sleep(3 * time.Second)

		for sleepMode {
			time.Sleep(10 * time.Second)
		}

		b := make([]byte, 102400)
		s, err := stdout.Read(b)
		if err == io.EOF && !sleepMode {
			time.Sleep(1 * time.Second)
			if sleepMode {
				continue
			}
			break
		}

		if b[0] != 0 {
			lines := bytes.Split(b[:s], []byte{'\n'})
			for _, line := range lines {
				if len(line) != 0 && line[0] != '*' && line[1] != '*' {
					if progressStep == 0 {
						regex.Comp(`(?mi)^>{1,3}\s*Emerging\s*\(?:\s*[0-9]+\s*of\s*([0-9]+)\s*\)`).RepFunc(line, func(data func(int) []byte) []byte {
							if i, e := strconv.Atoi(string(data(1))); e == nil {
								progressStep = progress / uint(i) / 3
							}
							return nil
						}, true)
					}

					regex.Comp(`(?mi)^>{1,3}\s*(Emerging|Installing|Completed)`).RepFunc(line, func(data func(int) []byte) []byte {
						chrootProgressAdd(progressStep)
						cProgress += progressStep
						return []byte{}
					}, true)

					fmt.Println(string(line))
				}
			}
		}
	}

	err = cmd.Wait()

	time.Sleep(1 * time.Second)

	chrootProgressRemove(cProgress)
	chrootProgressAdd(progress)

	chrootWaitToCool(false)

	if err != nil || installErr != nil {
		return errors.New("(chroot) error: failed to emerge @world")
	}

	fmt.Println("(chroot) finished emerging @world")

	// pretend to read the gentoo eselect news, so it stops bugging me to read it
	bash.Run([]string{`eselect`, `news`, `read`}, "", nil)

	return nil
}

func chrootInstallSELinux() error {
	// install selinux
	fmt.Println("(chroot) installing selinux...")

	_, err := bash.Run([]string{`emerge`, `--quiet`, `-1`, `sec-policy/selinux-base`}, "/", []string{`FEATURES=${FEATURES} -selinux`}, true, false)
	if err != nil {
		chrootProgressAdd(4000)
		return errors.New("(chroot) error: failed to install selinux")
	}

	chrootProgressAdd(1000)

	regex.Comp(`(?m)^SELINUX=.*$`).RepFileStr("/etc/selinux/config", []byte("SELINUX=permissive"), false)
	regex.Comp(`(?m)^SELINUXTYPE=.*$`).RepFileStr("/etc/selinux/config", []byte("SELINUXTYPE=strict"), false)

	_, err = bash.Run([]string{`emerge`, `--quiet`, `-1`, `sec-policy/selinux-base`}, "/", []string{`FEATURES=${FEATURES} -selinux -sesandbox`}, true, false)
	if err != nil {
		chrootProgressAdd(3000)
		return errors.New("(chroot) error: failed to install selinux")
	}

	chrootProgressAdd(1000)

	_, err = bash.Run([]string{`emerge`, `--quiet`, `-1`, `sec-policy/selinux-base-policy`}, "/", []string{`FEATURES=${FEATURES} -selinux -sesandbox`}, true, false)
	if err != nil {
		chrootProgressAdd(2000)
		return errors.New("(chroot) error: failed to install selinux")
	}

	chrootProgressAdd(1000)

	_, err = bash.Run([]string{`emerge`, `--quiet`, `sys-apps/policycoreutils`}, "/", []string{`FEATURES=${FEATURES} -selinux -sesandbox`}, true, false)
	if err != nil {
		chrootProgressAdd(1000)
		return errors.New("(chroot) error: failed to install selinux")
	}

	chrootProgressAdd(1000)

	bash.RunRaw(`setenforce 0`, "", nil)

	chrootWaitToCool(false)

	return nil
}

func chrootSetupSELinux(tarName string) {
	// enable selinux
	fmt.Println("(chroot) enabling selinux...")

	if !strings.Contains(tarName, "selinux") {
		os.MkdirAll("/mnt/gentoo", 0644)
		bash.Run([]string{`mount`, `-o`, `bind`, `/`, `/mnt/gentoo`}, "", nil)
		bash.RunRaw(`setfiles -r /mnt/gentoo /etc/selinux/strict/contexts/files/file_contexts /mnt/gentoo/{dev,home,proc,run,sys,tmp,games,var/home,var/roothome}`, "", nil, true)
		bash.Run([]string{`umount`, `-R`, `/mnt/gentoo`}, "", nil)
		bash.Run([]string{`rlpkg`, `-a`, `-r`}, "", nil)
		os.RemoveAll("/mnt/gentoo")

		chrootWaitToCool(false)
	}

	regex.Comp(`(?m)^SELINUX=.*$`).RepFileStr("/etc/selinux/config", []byte("SELINUX=enforcing"), false)

	bash.RunRaw(`setenforce 0`, "", nil)
}

func chrootSetup() error {
	// set cpu flags
	errList := install("app-portage/cpuid2cpuflags")
	if len(errList) != 0 {
		fmt.Println(errors.New("(chroot) error: failed to install app-portage/cpuid2cpuflags"))
	}else{
		_, err := bash.RunRaw(`echo "*/* $(cpuid2cpuflags)" > /etc/portage/package.use/00cpu-flags`, "/", nil)
		if err != nil {
			fmt.Println(errors.New("(chroot) error: failed to set cupflags"))
		}
	}

	chrootProgressAdd(1000)

	// install linux kernel
	err := chrootInstallKernel()
	if err != nil {
		chrootProgressAdd(5000)
		return err
	}

	// install filesystems
	errList = install(`sys-fs/e2fsprogs`, `sys-fs/dosfstools`, `sys-fs/xfsprogs`, `sys-fs/btrfs-progs`, `sys-fs/f2fs-tools`, `sys-fs/ntfs3g`)
	if len(errList) != 0 {
		pkgList := []string{}
		for name := range errList {
			pkgList = append(pkgList, name)
		}
		fmt.Println(errors.New("(chroot) error: failed to install: "+strings.Join(pkgList, " ")))
	}

	chrootProgressAdd(5000)

	return nil
}

func chrootInstallKernel() error {
	fmt.Println("(chroot) installing linux kernel...")

	errList := install(`sys-kernel/linux-firmware`)
	if len(errList) != 0 {
		chrootProgressAdd(2000)
		return errors.New("(chroot) error: failed to install sys-kernel/linux-firmware")
	}

	chrootProgressAdd(1000)

	errList = install(`sys-kernel/gentoo-kernel-bin`)
	if len(errList) != 0 {
		chrootProgressAdd(1000)
		return errors.New("(chroot) error: failed to install sys-kernel/gentoo-kernel-bin")
	}

	chrootProgressAdd(1000)

	return nil
}

func chrootSetupSystem(locale localeInfo, tarName string) error {
	err := regex.Comp(`(?m)^hostname=".*"$`).RepFileStr("/etc/conf.d/hostname", regex.JoinBytes(`hostname="`, bytes.ToLower([]byte(distroName)), '"'), false)
	if err != nil {
		appendToFile("/etc/conf.d/hostname", regex.JoinBytes(`hostname="`, bytes.ToLower([]byte(distroName)), '"'), 0644)
	}

	regex.Comp(`(?m)^(127\.0\.0\.1|::1)([\s\t ]+)localhost$`).RepFileFunc("/etc/hosts", func(data func(int) []byte) []byte {
		return regex.JoinBytes(data(1), data(2), bytes.ToLower([]byte(distroName)), ` localhost`)
	}, true)

	chrootProgressAdd(100)

	bash.Run([]string{`emerge`, `--noreplace`, `--quiet`, `net-misc/netifrc`}, "/", nil, true)

	chrootProgressAdd(1000)

	if buf, err := os.ReadFile("/etc/security/passwdqc.conf"); err == nil {
		buf = regex.Comp(`(?m)^min=.*$`).RepStrLit(buf, []byte("min=4,4,4,4,4"))
		buf = regex.Comp(`(?m)^max=.*$`).RepStrLit(buf, []byte("max=72"))
		buf = regex.Comp(`(?m)^passphrase=.*$`).RepStrLit(buf, []byte("passphrase=3"))
		buf = regex.Comp(`(?m)^match=.*$`).RepStrLit(buf, []byte("match=4"))
		buf = regex.Comp(`(?m)^similar=.*$`).RepStrLit(buf, []byte("similar=permit"))
		buf = regex.Comp(`(?m)^enforce=.*$`).RepStrLit(buf, []byte("enforce=everyone"))
		buf = regex.Comp(`(?m)^retry=.*$`).RepStrLit(buf, []byte("retry=3"))

		os.WriteFile("/etc/security/passwdqc.conf", buf, 0644)
	}

	bash.RunRaw(`passwd -d root`, "", []string{})

	regex.Comp(`(?m)^keymap=".*"$`).RepFileStr("/etc/conf.d/keymaps", regex.JoinBytes(`keymap="`, locale.keymap, '"'), false)

	chrootProgressAdd(100)

	//todo: detect if duel booting windows
	// may need to change from UTC to local if clock is off (I think I remember heaing somewhere, changing this may help if duel booting windows)
	// nano /etc/conf.d/hwclock

	installApps := []string{}
	if strings.Contains(tarName, "openrc") {
		// install logger
		installApps = append(installApps, `app-admin/sysklogd`)
	}else if strings.Contains(tarName, "systemd") {
		// enable time synchronization
		//todo: check if openrc handles something similar to timesyncd
		bash.Run([]string{`systemctl`, `enable`, `systemd-timesyncd`}, "/", nil, true)
	}

	appProgress := uint(0)

	// install cron
	installApps = append(installApps, `sys-process/cronie`)
	appProgress += 1000

	// file indexing
	installApps = append(installApps, `sys-apps/mlocate`)
	appProgress += 1000

	// bash completion
	installApps = append(installApps, `app-shells/bash-completion`)
	appProgress += 1000

	// recommended for nvme devices
	installApps = append(installApps, `sys-block/io-scheduler-udev-rules`)
	appProgress += 1000

	// install dhcp client and wireless tools
	installApps = append(installApps, `net-misc/dhcpcd`, `net-wireless/iw`, `net-wireless/wpa_supplicant`)
	appProgress += 1000


	// install partitioning tools
	installApps = append(installApps, `sys-apps/gptfdisk`, `sys-block/parted`)
	appProgress += 1000


	//todo: https://www.youtube.com/watch?v=k25TrKGXo_A

	// install app-admin/sudo
	installApps = append(installApps, `app-admin/sudo`)
	appProgress += 1000


	// install apps from list
	errList := install(installApps...)
	errPkg := []string{}
	for name := range errList {
		errPkg = append(errPkg, name)
	}
	if len(errPkg) != 0 {
		fmt.Println(errors.New("(chroot) error: failed to install: "+strings.Join(errPkg, " ")))
	}

	chrootProgressAdd(appProgress)

	// enable installed apps
	for _, pkg := range installApps {
		if goutil.Contains(errPkg, pkg) {
			continue
		}

		if strings.Contains(tarName, "openrc") {
			if pkg == "app-admin/sysklogd" {
				bash.Run([]string{`rc-update`, `add`, `sysklogd`, `default`}, "/", nil, true)
			}else if pkg == "sys-process/cronie" {
				bash.Run([]string{`rc-update`, `add`, `cronie`, `default`}, "/", nil, true)
			}else if pkg == "net-misc/dhcpcd" {
				bash.Run([]string{`rc-update`, `add`, `dhcpcd`, `default`}, "/", nil, true)
			}
		}else if strings.Contains(tarName, "systemd") {
			if pkg == "app-admin/sysklogd" {
				bash.Run([]string{`systemctl`, `enable`, `sysklogd`}, "/", nil, true)
			}else if pkg == "sys-process/cronie" {
				bash.Run([]string{`systemctl`, `enable`, `cronie`}, "/", nil, true)
			}else if pkg == "net-misc/dhcpcd" {
				bash.Run([]string{`systemctl`, `enable`, `dhcpcd`}, "/", nil, true)
			}
		}
	}

	chrootProgressAdd(appProgress/100)

	//todo: handle sshd setup for server install (Empoleos-v5-gentoo/scripts/core/chroot/system.sh)

	return nil
}


func chrootProgressAdd(progress uint){
	s := strconv.FormatUint(uint64(progress), 36)
	fmt.Println("CHROOT_PROGRESS:"+s)
}

func chrootProgressRemove(progress uint){
	s := strconv.FormatUint(uint64(progress), 36)
	fmt.Println("CHROOT_PROGRESS_RM:"+s)
}

func chrootProgressPrint(enable bool){
	if enable {
		fmt.Println("CHROOT_PROGRESS_PRINT:1")
	}else{
		fmt.Println("CHROOT_PROGRESS_PRINT:0")
	}
}

func chrootWaitToCool(strict bool){
	chrootProgressPrint(false)
	time.Sleep(100 * time.Millisecond)
	cputemp.WaitToCool(strict)
	time.Sleep(100 * time.Millisecond)
	chrootProgressPrint(true)
}
