package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex-re2/v2"
	"github.com/AspieSoft/goutil/bash"
	"github.com/AspieSoft/goutil/fs/v2"
	"github.com/AspieSoft/goutil/v7"
)

func setupInstall(diskParts diskPartList, cpu cpuType, tarName string) error {
	waitToCool(false)

	// setup fstab
	err := setupFStab(diskParts)
	if err != nil {
		return err
	}

	installProgress += 1000

	os.WriteFile("/mnt/gentoo/etc/profile.d/bash_ps.sh", regex.JoinBytes(
		`if [ "$PS1" ]; then`, '\n',
		`  PS1="\[\e[m\][\[\e[1;32m\]\u@\h\[\e[m\]:\[\e[1;34m\]\W\[\e[m\]]\[\e[0;31m\](\$?)\[\e[1;0m\]\\$ \[\e[m\]"`, '\n',
		`  #PS1="\[\e[1;33m\][\@]${PS1}" # show clock`, '\n',
		`fi`, '\n',
		'\n',
		`if [ "$EUID" = "0" -a "$(stat -c %d:%i /)" != "$(stat -c %d:%i /proc/1/root/. 2>/dev/null)" ]; then`, '\n',
		`  PS1="\[\e[1;35m\](chroot)\[\e[m\]${PS1}"`, '\n',
		`fi`,
	), 0644)

	bash.Run([]string{`ln`, `-sr`, `../../profile.d/bash_ps.sh`, `bash_ps.sh`}, "/mnt/gentoo/etc/bash/bashrc.d", nil)

	installProgress += 500

	//todo: consider adding cpu specific flags (https://wiki.gentoo.org/wiki/Safe_CFLAGS#Intel)
	// also remember the cpu_flags.yml file in the Empoleos-v5-gentoo project
	// regex.RepFileStr("/mnt/gentoo/etc/portage/make.conf", `(?m)^COMMON_FLAGS="(.*)"$`, []byte(`COMMON_FLAGS="-march=native $1"`), false)
	regex.Comp(`(?m)^COMMON_FLAGS="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
		if bytes.Contains(data(1), []byte("-march=native")) {
			return data(0)
		}
		return regex.JoinBytes(`COMMON_FLAGS="-march=native `, data(1), '"')
	}, false)

	installProgress += 500


	var cpuCores uint64
	if cpuInfo, err := bash.Run([]string{`lscpu`}, "", nil); err == nil {
		regex.Comp(`(?mi)^\s*CPU(?:\(s\)|)\s*:\s*([0-9]+)\s*$`).RepFunc(cpuInfo, func(data func(int) []byte) []byte {
			if i, err := strconv.Atoi(string(data(1))); err == nil {
				cpuCores = uint64(i)
			}
			return nil
		}, true)

		regex.Comp(`(?mi)^\s*Thread(?:\(s\)|)\s+per\s+core(?:\(s\)|)\s*:\s*([0-9]+)\s*$`).RepFunc(cpuInfo, func(data func(int) []byte) []byte {
			if i, err := strconv.Atoi(string(data(1))); err == nil {
				cpuCores /= uint64(i)
			}
			return nil
		}, true)

		if cpuCores == 0 {
			regex.Comp(`(?mi)^\s*CPU(?:\(s\)|)\s+family\s*:\s*([0-9]+)\s*$`).RepFunc(cpuInfo, func(data func(int) []byte) []byte {
				if i, err := strconv.Atoi(string(data(1))); err == nil {
					cpuCores = uint64(i)
				}
				return nil
			}, true)
		}
	}

	installProgress += 200

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

		memTotal /= 1000
		memTotal++
		memTotal /= 2
	}

	installProgress += 200

	if cpuCores == 0 || memTotal < cpuCores {
		cpuCores = memTotal
	}

	if cpuCores != 0 {
		hasOpt := false
		regex.Comp(`(?m)^MAKEOPT="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
			hasOpt = true
			if bytes.Contains(data(1), []byte("-j")) {
				return data(0)
			}
			return regex.JoinBytes(`MAKEOPT="-j`, cpuCores, ' ', data(1), '"')
		}, false)
		if !hasOpt {
			regex.Comp(`(?m)^FFLAGS=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
				if hasOpt {
					return data(0)
				}
				hasOpt = true
				return regex.JoinBytes(data(0), '\n', `MAKEOPT="-j`, cpuCores, '"')
			}, false)
		}
		if !hasOpt {
			regex.Comp(`(?m)^FCFLAGS=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
				if hasOpt {
					return data(0)
				}
				hasOpt = true
				return regex.JoinBytes(data(0), '\n', `MAKEOPT="-j`, cpuCores, '"')
			}, false)
		}
		if !hasOpt {
			regex.Comp(`(?m)^COMMON_FLAGS=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
				if hasOpt {
					return data(0)
				}
				hasOpt = true
				return regex.JoinBytes(data(0), '\n', `MAKEOPT="-j`, cpuCores, '"')
			}, false)
		}
		if !hasOpt {
			appendToFile("/mnt/gentoo/etc/portage/make.conf", regex.JoinBytes('\n', `MAKEOPT="-j`, cpuCores, '"', '\n'))
		}
	}

	hasOpt := false
	regex.Comp(`(?m)^ACCEPT_LICENSE="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
		hasOpt = true
		return []byte(`ACCEPT_LICENSE="*"`)
	}, false)
	if !hasOpt {
		regex.Comp(`(?m)^MAKEOPT=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
			hasOpt = true
			return regex.JoinBytes(data(0), '\n', `ACCEPT_LICENSE="*"`)
		}, false)
	}
	if !hasOpt {
		appendToFile("/mnt/gentoo/etc/portage/make.conf", regex.JoinBytes('\n', `ACCEPT_LICENSE="*"`, '\n'))
	}

	hasOpt = false
	regex.Comp(`(?m)^FEATURES="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
		hasOpt = true
		return regex.JoinBytes(`FEATURES="`, data(1), ` parallel-fetch`, `"`)
	}, false)
	if !hasOpt {
		regex.Comp(`(?m)^ACCEPT_LICENSE=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
			hasOpt = true
			return regex.JoinBytes(data(0), '\n', `FEATURES="${FEATURES} parallel-fetch"`)
		}, false)
	}
	if !hasOpt {
		appendToFile("/mnt/gentoo/etc/portage/make.conf", regex.JoinBytes('\n', `FEATURES="${FEATURES} parallel-fetch"`, '\n'))
	}

	installProgress += 200


	// add use flags
	// useFlags := []byte("boot gnuefi gcc cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl truetype python -apparmor -qtwebenging -webenging")
	// useFlags := []byte("boot gnuefi selinux gcc cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl truetype python -apparmor -qtwebenging -webenging")
	// useFlags := []byte("boot gnuefi selinux gcc make cmake cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib dist-kernel kernel-open modules modules-sign -apparmor -qt5 -qtwayland -qtwebenging -webenging")

	//todo: try without gcc use flag
	// useFlags := []byte("boot gnuefi selinux gcc make cmake cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib dist-kernel kernel-open modules modules-sign ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")
	// useFlags := []byte("boot gnuefi selinux make cmake cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib dist-kernel kernel-open modules modules-sign ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")
	//? or try with rust
	// useFlags := []byte("boot gnuefi selinux gcc make cmake rust cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib dist-kernel kernel-open modules modules-sign ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")
	//*added kernel-install
	// useFlags := []byte("boot gnuefi selinux gcc make cmake rust cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib kernel-install dist-kernel kernel-open modules modules-sign ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")
	//*removed cmake gcc rust
	// useFlags := []byte("boot gnuefi selinux make cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib kernel-install dist-kernel kernel-open modules modules-sign ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")
	//?removed kernel-install dist-kernel kernel-open modules modules-sign
	// useFlags := []byte("boot gnuefi selinux make cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")
	//?added -gcc -cmake
	// useFlags := []byte("-gcc -cmake boot gnuefi selinux make cryptsetup curl openssl pcre policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper efiemu mount nls sdl dns network tools truetype overlay container-init git python python3 sql go nodejs java brotli gzip zlib kernel-install dist-kernel kernel-open modules modules-sign ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svg xml corefonts fontconfig -apparmor -qt5 -qtwayland -qtwebenging -webenging")

	useFlags := regex.JoinBytes(
		// core
		"boot kernel-install dist-kernel kernel-open modules modules-sign gnuefi efiemu mount nls sdl selinux hardened policykit resolvconf seccomp sysv-utils tpm xfsprogs btrfs-progs device-mapper tools truetype overlay container-init -apparmor -qt5 -qtwayland -qtwebenging -webenging",
		// lang
		" make pcre curl git python python3 sql sqlite go nodejs java javascript ruby lua webkit",
		// security
		" cryptsetup openssl clamav crypt ssl session strip tcmalloc tcpd gcr skey -telemetry",
		// performance
		" jit smp sockets dri atm adns vc",
		// net
		" dns network ssh ftp sockets soap snmp ipv6 libwww sctp idn",
		// tools
		" utils encode emacs bash-completion geoip geolocation hddtemp scanner readline spell inotify dv",
		// media
		" ffmpeg multimedia opengl video sound png jpeg webp webm weba mp3 mp4 ogg wav svga xml corefonts fontconfig gif wavpack cgi fastcgi iconv aac flac tiff",
		// compression
		" brotli gzip zlib",
		// other
		" symlink cxx cvs magic icu acpi djvu expat exif tidy xattr",
	)

	//? secureboot

	// jit javascript ruby lua webkit
	// hardened clamav
	// utils -svg svga
	// magic cgi fastcgi
	// crypt encode emacs bash-completion geoip geolocation hddtemp wavpack ssl
	// scanner readline
	// session strip symlink
	// smp snmp soap sockets spell sqlite
	// ftp cxx cvs gif iconv icu idn inotify ipv6 libwww
	// dri atm adns acpi aac djvu expat exif dv
	// flac sctp
	// ssh tidy tiff xattr vc

	//security: tcmalloc tcpd -telemetry gcr skey

	//?desktop: dga calendar alsa accessibility aalib cups dvd dvdr jack lm-sensors portaudio samba screencast wifi wmf vulkan libnotify djvu ggi vorbis
	//?desktop: wine-proton wine


	/* if !installServer {
		// useFlags = append(useFlags, []byte(" xorg x11extras wayland gui joystick dbus bluetooth printsupport location ffmpeg multimedia opengl video sound png jpeg webp mp3 mp4 ogg wav svg xml corefonts fontconfig nvidia-drivers nvenc nsplugin")...)
		// useFlags = append(useFlags, []byte(" X xorg x11extras wayland xwayland gtk gui joystick dbus bluetooth printsupport location nvidia-drivers nvenc nsplugin")...)
		//* added positioning
		useFlags = append(useFlags, []byte(" X xorg x11extras wayland xwayland gtk gui joystick dbus bluetooth printsupport location positioning nvidia-drivers nvenc nsplugin")...)

		//!Note: This TODO may be done
		//todo: fix 'X gtk' USE flags not working
		// may try adding flag '--autounmask-write' to emerge
		// ACCEPT_KEYWORDS="~amd64"
		// /etc/portage/package.accept_keywords
		// X/X ~amd64
		// x11-base/xorg-drivers x11-base/xorg-apps gui-libs/gtk

		// dispatch-conf (u)
	}else{
		useFlags = append(useFlags, []byte(" monit nginx")...)
	} */

	if strings.Contains(tarName, "openrc") {
		useFlags = append(useFlags, []byte(" openrc -systemd")...)
	}else if strings.Contains(tarName, "systemd") {
		useFlags = append(useFlags, []byte(" systemd -openrc")...)
	}

	installProgress += 200

	//todo: handle adding desktop flags (may add these later)
	// useFlags = append(useFlags, []byte("dbus png jpeg webp gui gtk X libnotify -qt5 joystick opengl sound video bluetooth network multimedia printsupport location")...)
	// consider adding -netboot to desktop

	hasOpt = false
	regex.Comp(`(?m)^USE="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
		hasOpt = true
		newUse := bytes.Split(data(1), []byte{' '})
		for _, val := range bytes.Split(useFlags, []byte{' '}) {
			if !goutil.Contains(newUse, val) {
				newUse = append(newUse, val)
			}
		}
		return regex.JoinBytes(`USE="`, bytes.Join(newUse, []byte{' '}), '"')
	}, false)
	if !hasOpt {
		regex.Comp(`(?m)^ACCEPT_LICENSE=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
			hasOpt = true
			return regex.JoinBytes(data(0), '\n', `USE="`, useFlags, '"')
		}, false)
	}
	if !hasOpt {
		appendToFile("/mnt/gentoo/etc/portage/make.conf", regex.JoinBytes('\n', `USE="`, useFlags, '"', '\n'))
	}

	installProgress += 200


	// add grub platforms
	/* grubPlatforms := []byte("emu efi-32 efi-64 pc")

	hasOpt = false
	regex.Comp(`(?m)^GRUB_PLATFORMS="(.*)"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
		hasOpt = true
		newPlat := bytes.Split(data(1), []byte{' '})
		for _, val := range bytes.Split(grubPlatforms, []byte{' '}) {
			if !goutil.Contains(newPlat, val) {
				newPlat = append(newPlat, val)
			}
		}
		return regex.JoinBytes(`GRUB_PLATFORMS="`, bytes.Join(newPlat, []byte{' '}), '"')
	}, false)
	if !hasOpt {
		regex.Comp(`(?m)^USE=".*"$`).RepFileFunc("/mnt/gentoo/etc/portage/make.conf", func(data func(int) []byte) []byte {
			hasOpt = true
			return regex.JoinBytes(data(0), '\n', `GRUB_PLATFORMS="`, grubPlatforms, '"')
		}, false)
	}
	if !hasOpt {
		appendToFile("/mnt/gentoo/etc/portage/make.conf", regex.JoinBytes('\n', `GRUB_PLATFORMS="`, grubPlatforms, '"', '\n'))
	} */


	// copy dns info
	bash.Run([]string{`cp`, `--dereference`, `/etc/resolv.conf`, `/mnt/gentoo/etc/`}, "", nil)

	installProgress += 300

	return nil
}

func setupFStab(diskParts diskPartList) error {
	fstab := regex.JoinBytes(
		`# /etc/fstab: static file system information.`, '\n',
		`#`, '\n',
		`# See the manpage fstab(5) for more information.`, '\n',
		'\n',
	)

	bootUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.boot+` -lino UUID | head -n1`, "", nil)
	if err != nil || len(bootUUID) == 0 {
		return errors.New("error: failed to find uuid of boot partition")
	}
	fstab = regex.JoinBytes(fstab, `UUID=`, bytes.Trim(bootUUID, "   \r\n"), "\t\t\t\t\t", `/boot`, "\t\t", `vfat`, "\t\t", `noatime`, "\t\t", `0 2`, '\n')

	if diskParts.swap != "" {
		swapUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.swap+` -lino UUID | head -n1`, "", nil)
		if err != nil || len(swapUUID) == 0 {
			return errors.New("error: failed to find uuid of swap partition")
		}
		fstab = regex.JoinBytes(fstab, `UUID=`, bytes.Trim(swapUUID, "   \r\n"), '\t', `none`, "\t\t", `swap`, "\t\t", `sw`, "\t\t", `0 0`, '\n')
	}

	rootUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.root+` -lino UUID | head -n1`, "", nil)
	if err != nil || len(rootUUID) == 0 {
		return errors.New("error: failed to find uuid of root partition")
	}
	fstab = regex.JoinBytes(fstab, `UUID=`, bytes.Trim(rootUUID, "   \r\n"), '\t', `/`, "\t\t", `xfs`, "\t\t", `noatime`, "\t\t", `0 1`, '\n')

	if diskParts.home != "" {
		homeUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.home+` -lino UUID | head -n1`, "", nil)
		if err != nil || len(homeUUID) == 0 {
			return errors.New("error: failed to find uuid of home partition")
		}
		fstab = regex.JoinBytes(fstab, `UUID=`, bytes.Trim(homeUUID, "   \r\n"), '\t', `/var`, "\t\t", `btrfs`, "\t\t", `noatime`, "\t\t", `0 2`, '\n')
	}

	if diskParts.games != "" {
		gamesUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.games+` -lino UUID | head -n1`, "", nil)
		if err != nil || len(gamesUUID) == 0 {
			return errors.New("error: failed to find uuid of games partition")
		}
		fstab = regex.JoinBytes(fstab, `UUID=`, bytes.Trim(gamesUUID, "   \r\n"), '\t', `/games`, "\t\t", `ext4`, "\t\t", `noatime`, "\t\t", `0 2`, '\n')
	}

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
			fstab = append(fstab, []byte("\ntmpfs /tmp tmpfs rw,nosuid,noatime,nodev,size=16G,mode=1777 0 0\n")...)
		}
	}

	err = os.WriteFile("/mnt/gentoo/etc/fstab", fstab, 0644)
	if err != nil {
		return errors.New(`error: failed to write fstab file`)
	}

	return nil
}

func setupChroot(diskParts diskPartList, locale localeInfo, cpu cpuType, installDisk string, tarName string) error {
	waitToCool(false)

	// get root dir
	rootDir, err := filepath.Abs(".")
	if err != nil {
		rootDir = "."
	}

	os.MkdirAll("/mnt/gentoo/gentoo-installer", 0644)
	_, err = fs.Copy("gentoo-installer", "/mnt/gentoo/gentoo-installer/gentoo-installer")
	if err != nil {
		return err
	}

	installProgress += 100

	_, err = bash.Run([]string{`cp`, `-r`, `assets`, `/mnt/gentoo/gentoo-installer`}, rootDir, nil)
	if err != nil {
		return err
	}

	installProgress += 1000

	json, err := goutil.JSON.Stringify(map[string]interface{}{
		"installDisk": installDisk,
		"cpuType": cpu.cpu,
		"cpuType2": cpu.cpu2,
		"cpuReal": cpu.real,
		"timezone": locale.timezone,
		"continent": locale.continent,
		"locale": locale.locale,
		"keymap": locale.keymap,
		"tarName": tarName,
		"disk_boot": diskParts.boot,
		"disk_swap": diskParts.swap,
		"disk_root": diskParts.root,
		"disk_rootB": diskParts.rootB,
		"disk_home": diskParts.home,
		"disk_games": diskParts.games,
		"disk_mem": diskParts.mem,
		"installServer": installServer,
		"installUSB": installUSB,
	})
	if err != nil {
		return err
	}
	os.WriteFile("/mnt/gentoo/gentoo-installer/var.json", json, 0755)

	installProgress += 500

	_,  err = bash.Run([]string{`chmod`, `+x`, `/mnt/gentoo/gentoo-installer/gentoo-installer`}, "", nil)
	if err != nil {
		return err
	}

	// use bash file for chroot to ensure bash environment is available for commands
	err = os.WriteFile("/mnt/gentoo/gentoo-installer/run.sh", regex.JoinBytes(
		`#!/bin/bash`, '\n',
		'\n',
		`/gentoo-installer/gentoo-installer --chroot`,
	), 0755)
	if err != nil {
		return err
	}

	installProgress += 200

	waitToCool(false)

	// mounting necessary filesystems
	bash.Run([]string{`mount`, `--types`, `proc`, `/proc`, `/mnt/gentoo/proc`}, "", nil)
	bash.Run([]string{`mount`, `--rbind`, `/sys`, `/mnt/gentoo/sys`}, "", nil)
	bash.Run([]string{`mount`, `--make-rslave`, `/mnt/gentoo/sys`}, "", nil)
	bash.Run([]string{`mount`, `--rbind`, `/dev`, `/mnt/gentoo/dev`}, "", nil)
	bash.Run([]string{`mount`, `--make-rslave`, `/mnt/gentoo/dev`}, "", nil)
	bash.Run([]string{`mount`, `--bind`, `/run`, `/mnt/gentoo/run`}, "", nil)
	bash.Run([]string{`mount`, `--make-slave`, `/mnt/gentoo/run`}, "", nil)

	installProgress += 300

	// for non-gentoo install media
	bash.RunRaw(`test -L /dev/shm && rm /dev/shm && mkdir /dev/shm`, "", nil)
	bash.RunRaw(`mount --types tmpfs --options nosuid,nodev,noexec shm /dev/shm`, "", nil)
	bash.RunRaw(`chmod 1777 /dev/shm /run/shm`, "", nil)

	installProgress += 200

	waitToCool(false)


	// enter new environment and run chroot script
	cmd := exec.Command(`bash`, `-c`, `chroot /mnt/gentoo /bin/bash /gentoo-installer/run.sh`)
	cmd.Dir = "/mnt/gentoo"
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	var chErr error
	running := true
	go func(){
		for {
			buf := make([]byte, 1024)
			n, err := stdout.Read(buf)
			if err != nil {
				chErr = err
				running = false
				break
			}
			buf = buf[:n]

			buf = regex.Comp(`(?m)^CHROOT_PROGRESS:([A-Za-z0-9]+)\r?\n?`).RepFunc(buf, func(data func(int) []byte) []byte {
				if i, err := strconv.ParseUint(string(data(1)), 36, 64); err == nil {
					installProgress += uint(i)
				}
				return []byte{}
			})

			buf = regex.Comp(`(?m)^CHROOT_PROGRESS_RM:([A-Za-z0-9]+)\r?\n?`).RepFunc(buf, func(data func(int) []byte) []byte {
				if i, err := strconv.ParseUint(string(data(1)), 36, 64); err == nil {
					installProgress -= uint(i)
				}
				return []byte{}
			})

			buf = regex.Comp(`(?m)^CHROOT_PROGRESS_PRINT:([01])\r?\n?`).RepFunc(buf, func(data func(int) []byte) []byte {
				if b := data(1); len(b) != 0 {
					if b[0] == '1' {
						installProgressPrintEnabled = true
					}else if b[0] == '0' {
						installProgressPrintEnabled = false
					}
				}
				return []byte{}
			})

			if len(buf) != 0 || bytes.Equal(buf, []byte("\n")) || bytes.Equal(buf, []byte("\r\n")) {
				logData(string(buf), true)
			}
		}
	}()

	err = cmd.Run()
	if err != nil {
		return err
	}

	for running {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)

	if chErr != nil && chErr != io.EOF {
		return chErr
	}

	stdout.Close()


	waitToCool(false)

	// after chroot exits
	os.RemoveAll("/mnt/gentoo/gentoo-installer")

	installProgress += 100

	// unmount filesystems
	bash.RunRaw(`umount -l /mnt/gentoo/dev{/shm,/pts,}`, "", nil)
	bash.Run([]string{`umount`, `/mnt/gentoo/proc`}, "", nil)
	bash.Run([]string{`umount`, `/mnt/gentoo/sys`}, "", nil)
	bash.Run([]string{`umount`, `/mnt/gentoo/dev`}, "", nil)
	bash.Run([]string{`umount`, `/mnt/gentoo/run`}, "", nil)

	time.Sleep(1 * time.Second)

	installProgress += 100

	waitToCool(false)

	// cache new build
	/* bash.Run([]string{`tar`, `-cpf`, `cache/`+distroName+`.tar.xz`, `-C`, `/mnt/gentoo`, `.`, `--xattrs-include='*.*'`, `--numeric-owner`}, rootDir, nil)

	time.Sleep(1 * time.Second)

	installProgress += 5000

	// add checksum
	if out, err := bash.RunRaw(`sha256sum `+distroName+`.tar.xz`, "cache", nil); err == nil && len(out) != 0 {
		os.WriteFile("cache/"+distroName+".tar.xz.sum", out, 0644)
	} */

	time.Sleep(1 * time.Second)

	installProgress += 500

	waitToCool(false)

	// unmount disk
	unmountDisk(installDisk)

	installProgress += 100

	return nil
}
