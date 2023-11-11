package main

import "github.com/AspieSoft/goutil/bash"

func installChrootPrebuild(){
	config := initChroot()

	//todo: reconfigure system for prebuild file
	// may also consider running a user defined package list and bash script
	// may also allow user bash scripts to include .openrc|.systemd and .server|.desktop and possibly .usb

	// install bootloader
	err := chrootBootloaderSetup(config.installDisk, config.diskParts, config.cpu, config.tarName)
	if err != nil {
		panic(err)
	}

	chrootWaitToCool(false)

	// setup secureboot
	//todo: add secure boot for systemd-boot (https://wiki.gentoo.org/wiki/Systemd/systemd-boot)
	// also try to use redhat shim to simplify process
	err = secureBoot()
	if err != nil {
		panic(err)
	}

	// cleanup
	bash.RunRaw(`emerge --depclean --quiet`, "", nil)
	bash.RunRaw(`setsebool -P portage_use_nfs on`, "", nil)
}
