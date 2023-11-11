package main

func secureBoot() error {
	//todo: may need to run on normal boot (and not chroot)
	//note: wifi has not been figured out yet (may need to install packages before reboot)

	// mkdir -p /etc/portage/package.accept_keywords
	// echo 'sys-boot/mokutil ~amd64' >> /etc/portage/package.accept_keywords/mokutil

	// emerge sys-boot/shim sys-boot/mokutil

	// emerge sys-boot/refind

	//// cp /usr/share/shim/BOOTX64.EFI /boot/EFI/gentoo/BOOTX64.EFI
	//// cp /usr/share/shim/mmx64.efi /boot/EFI/gentoo/mmx64.efi
	// mv /boot/EFI/BOOT/BOOTX64.EFI /boot/EFI/BOOT/grubx64.efi
	// cp /usr/share/shim/BOOTX64.EFI /boot/EFI/BOOT/BOOTX64.EFI
	// cp /usr/share/shim/mmx64.efi /boot/EFI/BOOT/mmx64.efi

	//// USE=secureboot refind-install ... --shim /usr/share/shim/BOOTX64.EFI ...
	//// USE=secureboot refind-install --shim /usr/share/shim/BOOTX64.EFI

	// mv BOOTX64.EFI shimx64.efi
	//// ln -s /usr/share/shim/BOOTX64.EFI /usr/share/shim/shimx64.efi
	//// USE=secureboot refind-install --shim /usr/share/shim/shimx64.efi --yes

	//// USE=secureboot refind-install --shim /boot/EFI/BOOT/shimx64.efi --yes
	// USE=secureboot refind-install --shim /boot/EFI/BOOT/shimx64.efi --ownhfs /dev/mmcblk0p3 --alldrivers --yes
	// mv shimx64.efi BOOTX64.EFI

	//todo: test refind config
	// nano /boot/refind_linux.conf
	// c83ba3b1-8e7c-453d-a3ab-8286445c2f7f

	return nil
}
