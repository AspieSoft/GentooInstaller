package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AspieSoft/go-regex-re2/v2"
	"github.com/AspieSoft/goutil/bash"
)

func chrootBootloaderSetup(installDisk string, diskParts diskPartList, cpu cpuType, tarName string) error {
	//todo: use alternative to systemd-boot for non-efi systems
	// if [ -d /sys/firmware/efi ] (efi supported) else (efi not supported)
	// also detect if device is removable (USB/SD)
	// https://wiki.gentoo.org/wiki/Handbook:AMD64/Installation/Bootloader
	// also look at other boot loaders
	//todo: consider using 'lilo' for non-efi boot (and possibly for efi)
	//todo: consider using 'efibootmgr' for uefi
	// https://wiki.gentoo.org/wiki/Handbook:AMD64/Installation/Bootloader

	/* if strings.Contains(tarName, "openrc") {
		err = chrootSetupBootOpenRC(tarName)
		if err != nil {
			return err
		}
	}else if strings.Contains(tarName, "systemd") {
		err = chrootSetupBootSystemd(tarName)
		if err != nil {
			return err
		}
	} */

	if installUSB {
		return chrootSetupBoot_usb(installDisk, diskParts, cpu, tarName)
	}

	if _, e := os.Stat("/sys/firmware/efi"); e == nil {
		return chrootSetupBoot_efi(installDisk, diskParts, cpu, tarName)
	}

	return chrootSetupBoot_noefi(installDisk, diskParts, cpu, tarName)
}

func chrootSetupBoot_efi(installDisk string, diskParts diskPartList, cpu cpuType, tarName string) error {
	//todo: install grub or an allternative that works with efi
	// may also try efibootmgr to not have a secondary bootloader (if possible)
	// https://wiki.gentoo.org/wiki/Handbook:AMD64/Installation/Bootloader
	//!note: remember to test with secure boot
	return nil
}

func chrootSetupBoot_noefi(installDisk string, diskParts diskPartList, cpu cpuType, tarName string) error {
	fmt.Println("(chroot) installing sys-boot/lilo...")
	errList := install(`sys-boot/lilo`)
	if len(errList) != 0 {
		chrootProgressAdd(2500)
		return errors.New("(chroot) error: failed to install sys-boot/lilo")
	}

	chrootProgressAdd(1000)

	fmt.Println("(chroot) configuring lilo...")

	files, err := os.ReadDir("/boot")
	if err != nil {
		chrootProgressAdd(1500)
		return errors.New("(chroot) error: failed to read directory '/boot'")
	}

	var vmLinuz string
	var initRamFS string
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "vmlinuz") {
			vmLinuz = name
		}else if strings.HasPrefix(name, "initramfs") {
			initRamFS = name
		}
	}

	if vmLinuz == "" {
		chrootProgressAdd(1500)
		return errors.New("error: failed to find file '/boot/vmlinuz*'")
	}

	if initRamFS == "" {
		chrootProgressAdd(1500)
		return errors.New("error: failed to find file '/boot/initramfs*'")
	}

	vmLinuz = strings.Trim(vmLinuz, "\r\n ")
	initRamFS = strings.Trim(initRamFS, "\r\n ")

	chrootProgressAdd(100)

	rootUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.root+` -lino UUID | head -n1`, "", nil)
	if err != nil || len(rootUUID) == 0 {
		chrootProgressAdd(1400)
		return errors.New("error: failed to find uuid of root partition")
	}

	liloConf := regex.JoinBytes(
		`boot=`, installDisk, '\n',
		// `prompt`, '\n',
		// `timeout=50`, '\n',
		`default=`, distroName, '\n',
		`compact`, '\n',
		'\n',
		`image=`, `/boot/`, vmLinuz, '\n',
		`  label=`, distroName, '\n',
		`  read-only`, '\n',
		`  append="root=uuid="`, bytes.Trim(rootUUID, "\r\n "), '\n',
		`  initrd=/boot/`, initRamFS, '\n',
	)

	chrootProgressAdd(100)

	if diskParts.rootB != "" {
		rootUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.rootB+` -lino UUID | head -n1`, "", nil)
		if err != nil || len(rootUUID) == 0 {
			chrootProgressAdd(1300)
			return errors.New("error: failed to find uuid of root partition")
		}

		liloConf = append(liloConf, regex.JoinBytes(
			'\n',
		`image=`, `/boot/`, vmLinuz, '\n',
		`  label=`, distroName, `.back`, '\n',
		`  read-only`, '\n',
		`  append="root=uuid="`, bytes.Trim(rootUUID, "\r\n "), '\n',
		`  initrd=/boot/`, initRamFS, '\n',
		)...)
	}

	chrootProgressAdd(100)

	liloConf = append(liloConf, regex.JoinBytes(
		'\n',
		`image=`, `/boot/`, vmLinuz, '\n',
		`  label=`, distroName, `.rescue`, '\n',
		`  read-only`, '\n',
		`  append="root=uuid="`, bytes.Trim(rootUUID, "\r\n "), '\n',
		`  initrd=/boot/`, initRamFS, '\n',
		`  append="init=/bin/bb"`, '\n',
	)...)
	
	chrootProgressAdd(100)

	os.WriteFile("/etc/lilo.conf", liloConf, 0644)

	chrootProgressAdd(100)

	_, err = bash.Run([]string{`/sbin/lilo`}, "", nil, true, false)
	if err != nil {
		chrootProgressAdd(1000)
		return errors.New("(chroot) error: failed to install lilo config")
	}

	chrootProgressAdd(1000)

	return nil
}

func chrootSetupBoot_usb(installDisk string, diskParts diskPartList, cpu cpuType, tarName string) error {
	//todo: install syslinux for live usb bootloader
	// https://wiki.gentoo.org/wiki/Syslinux
	// https://wiki.gentoo.org/wiki/LiveUSB
	return nil
}


//!old

func chrootSetupBootOpenRC(tarName string) error {
	fmt.Println("(chroot) installing systemd-boot for openrc...")

	errList := install(`sys-apps/systemd-utils`)
	if len(errList) != 0 {
		chrootProgressAdd(3000)
		return errors.New("(chroot) error: failed to install sys-apps/systemd-utils")
	}

	chrootProgressAdd(1000)

	os.MkdirAll("/etc/portage/package.use", 0644)

	err := os.WriteFile("/etc/portage/package.use/systemd-utils", []byte("sys-apps/systemd-utils boot"), 0644)
	if err != nil {
		chrootProgressAdd(2000)
		return errors.New("(chroot) error: failed to write file '/etc/portage/package.use/systemd-utils'")
	}

	_, err = bash.Run([]string{`emerge`, `--oneshot`, `--quiet`, `sys-apps/systemd-utils`}, "/", nil, true, false)
	if len(errList) != 0 {
		chrootProgressAdd(2000)
		return errors.New("(chroot) error: failed to reinstall sys-apps/systemd-utils")
	}

	chrootProgressAdd(1000)

	_, err = bash.Run([]string{`bootctl`, `install`}, "/", nil, true, false)
	if len(errList) != 0 {
		chrootProgressAdd(1000)
		return errors.New("(chroot) error: failed to run 'bootctl install'")
	}

	chrootProgressAdd(1000)

	return nil
}

func chrootSetupBootSystemd(tarName string) error {
	fmt.Println("(chroot) installing systemd-boot for systemd...")

	errList := install(`sys-apps/systemd`)
	if len(errList) != 0 {
		chrootProgressAdd(3000)
		return errors.New("(chroot) error: failed to install sys-apps/systemd")
	}

	chrootProgressAdd(1000)

	os.MkdirAll("/etc/portage/package.use", 0644)

	err := os.WriteFile("/etc/portage/package.use/systemd", []byte("sys-apps/systemd boot"), 0644)
	if err != nil {
		chrootProgressAdd(2000)
		return errors.New("(chroot) error: failed to write file '/etc/portage/package.use/systemd'")
	}

	_, err = bash.Run([]string{`emerge`, `--oneshot`, `--quiet`, `sys-apps/systemd`}, "/", nil, true, false)
	if len(errList) != 0 {
		chrootProgressAdd(2000)
		return errors.New("(chroot) error: failed to reinstall sys-apps/systemd")
	}

	chrootProgressAdd(1000)

	_, err = bash.Run([]string{`bootctl`, `install`}, "/", nil, true, false)
	if len(errList) != 0 {
		chrootProgressAdd(1000)
		return errors.New("(chroot) error: failed to run 'bootctl install'")
	}

	chrootProgressAdd(1000)

	return nil
}

func chrootSetupBoot(diskParts diskPartList, tarName string) error {
	fmt.Println("(chroot) configuring systemd-boot...")

	files, err := os.ReadDir("/boot")
	if err != nil {
		chrootProgressAdd(1500)
		return errors.New("(chroot) error: failed to read directory '/boot'")
	}

	var vmLinuz string
	var initRamFS string
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "vmlinuz") {
			vmLinuz = name
		}else if strings.HasPrefix(name, "initramfs") {
			initRamFS = name
		}
	}

	if vmLinuz == "" {
		chrootProgressAdd(1500)
		return errors.New("error: failed to find file '/boot/vmlinuz*'")
	}

	if initRamFS == "" {
		chrootProgressAdd(1500)
		return errors.New("error: failed to find file '/boot/initramfs*'")
	}

	vmLinuz = strings.Trim(vmLinuz, "\r\n ")
	initRamFS = strings.Trim(initRamFS, "\r\n ")

	chrootProgressAdd(100)

	rootUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.root+` -lino UUID | head -n1`, "", nil)
	if err != nil || len(rootUUID) == 0 {
		chrootProgressAdd(1400)
		return errors.New("error: failed to find uuid of root partition")
	}

	os.WriteFile("/boot/loader/entries/"+strings.ToLower(distroName)+".conf", regex.JoinBytes(
		`title `, distroName, '\n',
		`linux /`, vmLinuz, '\n',
		`initrd /`, initRamFS, '\n',
		`options root="UUID=`, bytes.Trim(rootUUID, "\r\n "), `" quiet`, '\n',
	), 0644)

	chrootProgressAdd(200)

	if diskParts.rootB != "" {
		rootUUID, err := bash.RunRaw(`lsblk /dev/`+diskParts.rootB+` -lino UUID | head -n1`, "", nil)
		if err != nil || len(rootUUID) == 0 {
			chrootProgressAdd(1200)
			return errors.New("error: failed to find uuid of root partition")
		}

		os.WriteFile("/boot/loader/entries/"+strings.ToLower(distroName)+"-back.conf", regex.JoinBytes(
			`title `, distroName, ` (Back)`, '\n',
			`linux /`, vmLinuz, '\n',
			`initrd /`, initRamFS, '\n',
			`options root="UUID=`, bytes.Trim(rootUUID, "\r\n "), `" quiet`, '\n',
		), 0644)
	}

	chrootProgressAdd(200)

	//todo: find out how to set default boot entry (systemd-boot)

	errList := install(`sys-kernel/installkernel-systemd-boot`)
	if len(errList) != 0 {
		chrootProgressAdd(1000)
		return errors.New("(chroot) error: failed to install sys-kernel/installkernel-systemd-boot")
	}

	chrootProgressAdd(1000)

	return nil
}
