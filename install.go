package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex-re2/v2"
	"github.com/AspieSoft/goutil/bash"
	"github.com/AspieSoft/goutil/fs/v2"
	"github.com/AspieSoft/goutil/v7"
)

var dataSizeChars = []string{
	"b",
	"kb",
	"mb",
	"gb",
	"tb",
	"pb",
	"eb",
	"zb",
	"yb",
}

type diskPartList struct {
	boot string
	swap string
	root string
	rootB string
	home string
	games string

	mem uint64
}


func installGentoo(locale localeInfo, cpu cpuType, installDisk string, gameSize uint64, sysinit string) {
	//todo: get max progress from 'installProgress' var when install is done
	maxProgress := uint(0)
	// maxProgress := uint(301450)

	runningInstall := true
	go func(){
		lastProgress := uint(0)
		lastMaxSize := 0
		lastPrintStr := ""

		for {
			time.Sleep(100 * time.Nanosecond)

			if !runningInstall {
				break
			}

			if !installProgressPrintEnabled {
				continue
			}

			maxSize := 50
			if out, err := bash.Run([]string{`tput`, `cols`}, "", nil); err == nil {
				out = bytes.Trim(out, " \r\n")
				if i, err := strconv.Atoi(string(out)); err == nil && i != 0 && i > 5 {
					maxSize = i - 5
				}
			}
			maxTerminalSize = maxSize
			maxSize -= 4

			if installProgress == lastProgress && maxSize == lastMaxSize {
				fmt.Print(lastPrintStr)
				continue
			}

			var printStr string
			if maxProgress == 0 || maxSize < 20 {
				printStr = " Installing... "+strconv.FormatFloat(float64(installProgress) / 1000, 'f', 2, 64)+"                    "
			}else{
				progressPercent := float64(installProgress) / float64(maxProgress) * 100

				printStr = " Installing... "+strconv.FormatFloat(progressPercent, 'f', 2, 64)+"%"
				if len(printStr) + 10 > maxSize {
					printStr += strings.Repeat(" ", len(printStr)-maxSize)
				}else{
					pMax := maxSize-len(printStr)
					pSize := int(progressPercent/100*float64(pMax))
					if pSize > pMax {
						pSize = pMax
					}
					printStr += " ["+strings.Repeat("=", pSize)+strings.Repeat("-", pMax-pSize)+"]"
					printStr += strings.Repeat(" ", len(printStr)-maxSize)
				}
			}

			printStr += "\r"

			fmt.Print(printStr)

			lastProgress = installProgress
			lastMaxSize = maxSize
			lastPrintStr = printStr
		}
	}()

	resolveCPUType2(&cpu)

	installProgress += 200

	waitToCool(false)

	tarName, tarball, err := findTarball(cpu, sysinit)
	if err != nil {
		runningInstall = false
		logErr(errors.New("error: failed to find installation file"), true)
		return
	}
	logData("Found Gentoo Tarball: "+tarName)

	// format disk
	unmountDisk(installDisk)
	diskParts, err := wipeDisk(installDisk, gameSize, false) //todo: ask user if they would like to include rootB partition (and make root imutable)
	if err != nil {
		runningInstall = false
		logErr(err, true)
		return
	}

	// mount disk partitions
	mountDisk(diskParts)

	installProgress += 100

	// install gentoo tarball
	prebuild, err := installTarball(cpu, tarName, tarball)
	if err != nil {
		runningInstall = false
		logErr(err, true)
		return
	}

	if !prebuild {
		cProgress := installProgress

		// setup install
		err = setupInstall(diskParts, cpu, tarName)
		if err != nil {
			runningInstall = false
			logErr(err, true)
			return
		}

		// setup and run chroot
		err = setupChroot(diskParts, locale, cpu, installDisk, tarName)
		if err != nil {
			runningInstall = false
			logErr(err, true)
			return
		}

		installProgress += 1000

		//todo: add extra progress for prebuild difference
		fmt.Println("(debug) !prebuild Progress: "+strconv.FormatUint(uint64(installProgress - cProgress), 10))
	}else{
		installProgress += 301450

		//todo: correct changes to cpu, locale, and fstab
		// also run installChrootPrebuild in chroot (triggered by flag '--chroot-pre')
	}

	//todo: clone root partition to rootB (if needed)
	// also remember to update fstab file to include the new root partition uuid

	fmt.Println("(debug) Progress: "+strconv.FormatUint(uint64(installProgress), 10))

	runningInstall = false
}


func wipeDisk(installDisk string, gameSize uint64, includeRootB bool) (diskPartList, error) {
	bootSize := uint64(512)
	swapSize := uint64(16000)
	rootSize := uint64(16000)
	homeSize := uint64(128000)


	var memTotal uint64
	if out, err := bash.Run([]string{`grep`, `MemTotal`, `/proc/meminfo`}, "/", nil); err == nil {
		out = regex.Comp(`^.*?\s*([0-9]+)\s*([A-Za-z]*)\s*.*$`).RepStr(out, []byte("$1 $2"))
		data := bytes.SplitN(out, []byte{' '}, 2)
		if len(data) > 0 {
			if len(data) == 1 {
				data = append(data, []byte{'b'})
			}
			if mem, err := strconv.Atoi(string(data[0])); err == nil {
				t := string(bytes.ToLower(data[1]))

				if mbInd, err := goutil.IndexOf(dataSizeChars, "mb"); err == nil {
					if i, err := goutil.IndexOf(dataSizeChars, t); err == nil {
						for i > mbInd {
							mem *= 1000
							i--
						}
						for i < mbInd {
							mem /= 1000
							i++
						}

						if mem > 0 {
							memTotal = uint64(mem)
						}
					}
				}
			}
		}
	}

	installProgress += 100

	swapSize -= memTotal

	var diskTotal uint64 = 16000
	if out, err := bash.Run([]string{`lsblk`, `-nlo`, `size`, `/dev/`+installDisk}, "/", nil); err == nil {
		regex.Comp(`(?m)^.*?([0-9]+)(?:\.[0-9]+|)([BKMGTPEZY]|).*?$`).RepFunc(out, func(data func(int) []byte) []byte {
			if size, err := strconv.Atoi(string(data(1))); err == nil {
				t := string(bytes.ToLower(data(2)))
				t += "b"
				if t == "bb" {
					t = "b"
				}

				if mbInd, err := goutil.IndexOf(dataSizeChars, "mb"); err == nil {
					if i, err := goutil.IndexOf(dataSizeChars, t); err == nil {
						for i > mbInd {
							size *= 1000
							i--
						}
						for i < mbInd {
							size /= 1000
							i++
						}

						if size > 0 {
							diskTotal = uint64(size)
						}
					}
				}
			}

			return nil
		}, true)
	}

	installProgress += 100

	diskTotal -= bootSize

	if diskTotal > 256000 {
		rootSize = 32000
	}else if diskTotal < 128000 {
		rootSize = 8000
	}

	if swapSize < 2000 && swapSize > 0 {
		swapSize = 2000
	}else if swapSize < 0 {
		swapSize = 0
	}

	if diskTotal < 16000 {
		swapSize = 0
	}else if diskTotal < 64000 && swapSize > 2000 {
		swapSize = 2000
	}else if diskTotal < 128000 && swapSize > 4000 {
		swapSize = 4000
	}else if diskTotal < 256000 && swapSize > 8000 {
		swapSize = 8000
	}else if swapSize > 16000 {
		swapSize = 16000
	}

	//todo: consider adding tempfs partition for /tmp directory (https://wiki.gentoo.org/wiki/Tmpfs)
	// also consider the amount of ram and disk space, to determine if this partition should be created


	diskTotal -= rootSize
	diskTotal -= swapSize
	diskTotal -= homeSize

	if diskTotal < 0 {
		homeSize = 0
		gameSize = 0
	}else{
		if gameSize > diskTotal {
			gameSize = diskTotal
		}
	}

	//todo: make rootB optional by user
	var rootB bool
	if rootSize >= 16000 && includeRootB {
		rootSize /= 2
		rootB = true
	}

	installProgress += 100


	waitToCool(false)


	logData("wiping disk...")
	bash.Run([]string{`wipefs`, `--all`, `/dev/`+installDisk}, "", nil)
	installProgress += 2000
	bash.Run([]string{`mkfs.ext4`, `/dev/`+installDisk}, "", nil)
	installProgress += 1000
	bash.Run([]string{`wipefs`, `--all`, `/dev/`+installDisk}, "", nil)
	installProgress += 2000


	waitToCool(false)


	logData("setting up disk...")
	cmd := exec.Command(`parted`, `-a`, `optimal`, `/dev/`+installDisk)

	installProgress += 200

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return diskPartList{}, err
	}
	defer stdin.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return diskPartList{}, err
	}
	defer stdout.Close()


	cmdList := []string{
		`mktable gpt`,
		`unit mib`,
	}

	installProgress += 10

	start := uint64(1)
	i := 1

	// boot
	cmdList = append(cmdList,
		`mkpart primary `+strconv.FormatUint(start, 10)+` `+strconv.FormatUint(start+bootSize, 10),
		`name `+strconv.Itoa(i)+` boot`,
	)
	start += bootSize
	i++

	installProgress += 20

	if swapSize != 0 {
		// swap
		cmdList = append(cmdList,
			`mkpart primary `+strconv.FormatUint(start, 10)+` `+strconv.FormatUint(start+swapSize, 10),
			`name `+strconv.Itoa(i)+` swap`,
			`set `+strconv.Itoa(i)+` swap on`,
		)
		start += swapSize
		i++
	}

	installProgress += 20

	if homeSize == 0 {
		// root
		cmdList = append(cmdList,
			`mkpart primary `+strconv.FormatUint(start, 10)+` -1`,
			`name `+strconv.Itoa(i)+` root`,
		)
		i++

		rootB = false

		installProgress += 80
	}else{
		// root
		if rootB {
			cmdList = append(cmdList,
				`mkpart primary `+strconv.FormatUint(start, 10)+` `+strconv.FormatUint(start+rootSize, 10),
				`name `+strconv.Itoa(i)+` rootA`,
			)
		}else{
			cmdList = append(cmdList,
				`mkpart primary `+strconv.FormatUint(start, 10)+` `+strconv.FormatUint(start+rootSize, 10),
				`name `+strconv.Itoa(i)+` root`,
			)
		}
		start += rootSize
		i++

		installProgress += 20

		if rootB {
			cmdList = append(cmdList,
				`mkpart primary `+strconv.FormatUint(start, 10)+` `+strconv.FormatUint(start+rootSize, 10),
				`name `+strconv.Itoa(i)+` rootB`,
			)
			start += rootSize
			i++
		}

		installProgress += 20

		if gameSize == 0 {
			// home
			cmdList = append(cmdList,
				`mkpart primary `+strconv.FormatUint(start, 10)+` -1`,
				`name `+strconv.Itoa(i)+` home`,
			)
			i++

			installProgress += 40
		}else{
			// home
			cmdList = append(cmdList,
				`mkpart primary `+strconv.FormatUint(start, 10)+` -`+strconv.FormatUint(gameSize, 10),
				`name `+strconv.Itoa(i)+` home`,
			)
			i++

			installProgress += 20

			// games
			cmdList = append(cmdList,
				`mkpart primary -`+strconv.FormatUint(gameSize, 10)+` -1`,
				`name `+strconv.Itoa(i)+` games`,
			)
			i++

			installProgress += 20
		}
	}

	if _, e := os.Stat("/sys/firmware/efi"); e == nil {
		cmdList = append(cmdList,
			`set 1 boot on`,
		)
	}else{
		cmdList = append(cmdList,
			`set 1 bios_grub on`,
			// `set 1 bls_boot on`,
			`set 1 legacy_boot on`,
		)
	}

	cmdList = append(cmdList,
		`quit`,
	)

	installProgress += 10

	progress := uint(2000)

	progressStep := progress / uint(len(cmdList[i]))
	cProgress := installProgress

	var cmdErr error

	// create partitions
	go func(){
		i := 0
		waiting := int64(0)

		for {
			buf := make([]byte, 1024)
			n, err := stdout.Read(buf)
			if err != nil {
				break
			}
			buf = buf[:n]

			regex.Comp(`\(?parted\)?\s*$`).Match(buf)
			if regex.Comp(`\(?parted\)?[\?:]?\s*$`).Match(buf) {
				stdin.Write([]byte(cmdList[i]+"\n"))
				i++
				waiting = 0
				installProgress += progressStep
			}else if regex.Comp(`\(?[Yy]es[\\/][Nn]o\??\)?[\?:]?\s*$`).Match(buf) {
				stdin.Write([]byte("yes\n"))
				waiting = 0
			}else{
				time.Sleep(10 * time.Millisecond)
				if waiting == 0 {
					waiting = time.Now().UnixMilli()
				}else if time.Now().UnixMilli() - waiting > 60000 {
					logData(string(buf))
					cmdErr = errors.New("error: parted opporation not handled")
					return
				}
			}
		}
	}()

	cmd.Run()

	stdin.Close()
	stdout.Close()

	if cmdErr != nil {
		return diskPartList{}, cmdErr
	}

	installProgress = cProgress + progress


	waitToCool(false)


	logData("creating partition filesystems...")

	out, err := bash.Run([]string{`lsblk`, `-nlo`, `name,partlabel`, `/dev/`+installDisk}, "", nil)
	if err != nil {
		return diskPartList{}, err
	}

	diskParts := diskPartList{
		mem: memTotal,
	}

	progress = 2000

	progressStep = progress / 6
	cProgress = installProgress

	regex.Comp(`(?m)^(%1[\w_-]+)\s+([\w_-]+)$`, installDisk).RepFunc(out, func(data func(int) []byte) []byte {
		logData(" - creating "+string(data(2))+" partition...")
		installProgress += progressStep

		var partType string
		if bytes.Equal(data(2), []byte("boot")) {
			partType = `mkfs.vfat`
			diskParts.boot = string(data(1))
		}else if bytes.Equal(data(2), []byte("root")) || bytes.Equal(data(2), []byte("rootA")) || bytes.Equal(data(2), []byte("rootB")) {
			partType = `mkfs.xfs`
			if bytes.Equal(data(2), []byte("rootB")) {
				diskParts.rootB = string(data(1))
			}else{
				diskParts.root = string(data(1))
			}
		}else if bytes.Equal(data(2), []byte("home")) {
			partType = `mkfs.btrfs`
			diskParts.home = string(data(1))
		}else if bytes.Equal(data(2), []byte("games")) {
			partType = `mkfs.ext4`
			diskParts.games = string(data(1))
		}else if bytes.Equal(data(2), []byte("swap")) {
			partType = `mkswap`
			diskParts.swap = string(data(1))
		}

		if partType != "" {
			bash.Run([]string{partType, `/dev/`+string(data(1))}, "", nil)

			if bytes.Equal(data(2), []byte("boot")) {
				bash.Run([]string{`fatlabel`, `/dev/`+string(data(1)), strings.ToUpper(distroName)}, "", nil)
			}
		}

		return []byte{}
	}, true)

	installProgress = cProgress + progress


	logData("Finished Partitioning Disk")

	waitToCool(false)

	return diskParts, nil
}

func installTarball(cpu cpuType, tarName, tarball string) (prebuild bool, err error) {
	isPrebuild := false

	logData("Installing Tarball: "+tarName+"...")

	if stat, err := os.Stat("cache/"+distroName+".tar.xz"); err == nil && !stat.IsDir() {
		logData("Found "+distroName+".tar.xz in the cache")

		// verify checksum
		if _, err := bash.RunRaw(`sha256sum -c `+distroName+`.tar.xz.sum`, "cache", nil); err != nil {
			installProgress += 10000
			return false, errors.New("error: tarball failed to verify checksum sha256!")
		}

		_, err := fs.Copy("cache/"+distroName+".tar.xz", "/mnt/gentoo/"+distroName+".tar.xz")
		if err != nil {
			installProgress += 10000
			return true, err
		}
		isPrebuild = true

		installProgress += 5000
	}else if stat, err := os.Stat("cache/"+tarball); err == nil && !stat.IsDir() {
		logData("Found "+tarball+" in the cache")

		// verify checksum
		if _, err := bash.RunRaw(`sha256sum -c `+tarball+`.sum`, "cache", nil); err != nil {
			installProgress += 10000
			return false, errors.New("error: tarball failed to verify checksum sha256")
		}

		_, err := fs.Copy("cache/"+tarball, "/mnt/gentoo/"+tarball)
		if err != nil {
			installProgress += 10000
			return false, err
		}

		installProgress += 5000
	}else{
		logData("Downloading "+tarball+" from gentoo.org")

		url := "https://distfiles.gentoo.org/releases/"+cpu.cpu+"/autobuilds/current-stage3-"+cpu.cpu2+"-"+tarName+"/"+tarball

		delFiles := func(main bool){
			if main {
				os.Remove("/mnt/gentoo/"+tarball)
			}

			os.Remove("/mnt/gentoo/"+tarball+".sha256")
			os.Remove("/mnt/gentoo/"+tarball+".CONTENTS.gz")
			os.Remove("/mnt/gentoo/"+tarball+".DIGESTS")
			os.Remove("/mnt/gentoo/"+tarball+".asc")
		}

		progress := uint(4000)

		progressStep := progress / 7
		cProgress := installProgress

		retries := 3
		for {
			//todo: log output to logData function
			installProgressPrintEnabled = false
			_, err := bash.Run([]string{`wget`, url}, "/mnt/gentoo", nil, true)
			installProgressPrintEnabled = true
			if err != nil {
				delFiles(true)
				installProgress = cProgress + 10000
				return false, errors.New("error: tarball failed to download tarball")
			}

			installProgress += 1000

			_, err = bash.Run([]string{`wget`, url+".sha256"}, "/mnt/gentoo", nil)
			if err != nil {
				delFiles(true)
				installProgress = cProgress + 10000
				return false, errors.New("error: tarball failed to download checksum sha256")
			}

			installProgress += progressStep

			_, err = bash.Run([]string{`sha256sum`, `-c`, tarball+".sha256"}, "/mnt/gentoo", nil)
			if err != nil {
				if retries <= 0 {
					delFiles(true)
					installProgress = cProgress + 10000
					return false, errors.New("error: tarball failed to verify checksum sha256")
				}
				retries--
				installProgress = cProgress
				continue
			}

			installProgress += progressStep

			var noContents bool
			_, err = bash.Run([]string{`wget`, url+".CONTENTS.gz"}, "/mnt/gentoo", nil)
			if err != nil {
				/* delFiles(true)
				installProgress = cProgress + 10000
				return false, errors.New("error: tarball failed to download checksum CONTENTS.gz") */
				noContents = true
			}

			installProgress += progressStep

			var noDigest bool
			_, err = bash.Run([]string{`wget`, url+".DIGESTS"}, "/mnt/gentoo", nil)
			if err != nil {
				/* delFiles(true)
				installProgress = cProgress + 10000
				return false, errors.New("error: tarball failed to download checksum DIGESTS") */
				noDigest = true
			}

			installProgress += progressStep

			var noASC bool
			_, err = bash.Run([]string{`wget`, url+".asc"}, "/mnt/gentoo", nil)
			if err != nil {
				/* delFiles(true)
				installProgress = cProgress + 10000
				return false, errors.New("error: tarball failed to download checksum asc") */
				noASC = true
			}

			installProgress += progressStep

			if !noASC {
				bash.Pipe("/mnt/gentoo", []string{`wget`, `-O`, `-`, `https://qa-reports.gentoo.org/output/service-keys.gpg`}, []string{`gpg`, `--import`})

				_, err = bash.Run([]string{`gpg`, `--verify`, tarball+".asc"}, "/mnt/gentoo", nil)
				if err != nil {
					if retries <= 0 {
						delFiles(true)
						installProgress = cProgress + 10000
						return false, errors.New("error: tarball failed to verify checksum asc")
					}
					retries--
					installProgress = cProgress
					continue
				}
			}

			installProgress += progressStep

			if !noDigest {
				checkSums, err := bash.Run([]string{`sha512sum`, `-c`, tarball+".DIGESTS"}, "/mnt/gentoo", nil)
				if err != nil && (!bytes.Contains(checkSums, []byte(tarball+": OK")) || (!noContents && !bytes.Contains(checkSums, []byte(tarball+".CONTENTS.gz: OK")))) {
					if retries <= 0 {
						delFiles(true)
						installProgress = cProgress + 10000
						return false, errors.New("error: tarball failed to verify checksum DIGESTS")
					}
					retries--
					installProgress = cProgress
					continue
				}
			}

			if noContents && noDigest && noASC {
				if retries <= 0 {
					delFiles(true)
					installProgress = cProgress + 10000
					return false, errors.New("error: tarball failed to download checksums: CONTENTS.gz, DIGESTS, asc")
				}
				retries--
				installProgress = cProgress
				continue
			}else if noContents || noDigest || noASC {
				errMsg := []string{}
				if noContents {
					errMsg = append(errMsg, "CONTENTS.gz")
				}
				if noDigest {
					errMsg = append(errMsg, "DIGESTS")
				}
				if noASC {
					errMsg = append(errMsg, "asc")
				}
				logErr(errors.New("warning: tarball failed to download checksums: "+strings.Join(errMsg, ", ")))
			}

			installProgress += progressStep

			delFiles(false)

			bash.Run([]string{`rm`, `-f`, string(regex.Comp(`[A-Za-z0-9]+\.tar\.xz$`).RepStrLit([]byte(tarball), []byte{}))+"*"}, "cache", nil)
			os.MkdirAll("cache", 0644)
			bash.RunRaw(`rm -f stage3-`+cpu.cpu2+`-`+tarName+`-*`, "cache", nil)
			fs.Copy("/mnt/gentoo/"+tarball, "cache/"+tarball)

			// add checksum
			if out, err := bash.RunRaw(`sha256sum `+tarball, "cache", nil); err == nil && len(out) != 0 {
				os.WriteFile("cache/"+tarball+".sum", out, 0644)
			}

			break
		}

		installProgress = cProgress + 5000
	}

	logData("Installing Tarball: "+tarName+"...")
	_, err = bash.Run([]string{`tar`, `xpf`, tarball, `--xattrs-include='*.*'`, `--numeric-owner`}, "/mnt/gentoo", nil)
	os.Remove("/mnt/gentoo/"+tarball)

	installProgress += 5000

	return isPrebuild, err
}

func mountDisk(diskParts diskPartList) error {
	// mound disk
	err := os.MkdirAll("/mnt/gentoo", 0555)
	if err != nil {
		return err
	}

	_, err = bash.Run([]string{`mount`, `/dev/`+diskParts.root, `/mnt/gentoo`}, "", nil)
	if err != nil {
		return err
	}
	os.MkdirAll("/mnt/gentoo/boot", 0555)
	_, err = bash.Run([]string{`mount`, `/dev/`+diskParts.boot, `/mnt/gentoo/boot`}, "", nil)
	if err != nil {
		return err
	}

	if diskParts.home != "" {
		os.MkdirAll("/mnt/gentoo/var", 0644)
		_, err = bash.Run([]string{`mount`, `/dev/`+diskParts.home, `/mnt/gentoo/var`}, "", nil)
		if err != nil {
			return err
		}
	}

	if diskParts.games != "" {
		os.MkdirAll("/mnt/gentoo/games", 0644)
		_, err = bash.Run([]string{`mount`, `/dev/`+diskParts.games, `/mnt/gentoo/games`}, "", nil)
		if err != nil {
			return err
		}
	}

	if diskParts.swap != "" {
		_, err = bash.Run([]string{`swapon`, `/dev/`+diskParts.swap}, "", nil)
		if err != nil {
			return err
		}
	}

	time.Sleep(1 * time.Second)
	waitToCool(false)

	return nil
}

func unmountDisk(installDisk string){
	bash.Run([]string{`umount`, `-R`, `/mnt/gentoo`}, "", nil)
	bash.Pipe("", []string{`bash`, `-c`, `ls /dev/`+installDisk+`*`}, []string{`xargs`, `-n1`, `umount`, `-l`})
	bash.Pipe("", []string{`bash`, `-c`, `ls /dev/`+installDisk+`*`}, []string{`xargs`, `-n1`, `swapoff`})

	time.Sleep(1 * time.Second)
}
