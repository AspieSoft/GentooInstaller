#!/bin/bash

disk="mmcblk0"
diskP="${disk}p"

# unmount disk
umount -l /mnt/gentoo/dev{/shm,/pts,}
umount -R /mnt/gentoo

ls /dev/${disk}* | xargs -n1 umount -l &>/dev/null
ls /dev/${disk}* | xargs -n1 swapoff &>/dev/null

if [ "$1" == "close" -o "$1" == "exit" -o "$1" == "end" -o "$1" == "e" -o "$1" == "c" -o "$1" == "-e" -o "$1" == "c" -o "$1" == "--exit" -o "$1" == "--close" ]; then
  exit
fi

# mount disk
mount /dev/${diskP}3 /mnt/gentoo
mount /dev/${diskP}1 /mnt/gentoo/boot
mount /dev/${diskP}4 /mnt/gentoo/var
mount /dev/${diskP}5 /mnt/gentoo/games

# mounting necessary filesystems
mount --types proc /proc /mnt/gentoo/proc
mount --rbind /sys /mnt/gentoo/sys
mount --make-rslave /mnt/gentoo/sys
mount --rbind /dev /mnt/gentoo/dev
mount --make-rslave /mnt/gentoo/dev
mount --bind /run /mnt/gentoo/run
mount --make-slave /mnt/gentoo/run

# for non-gentoo install media
test -L /dev/shm && rm /dev/shm && mkdir /dev/shm
mount --types tmpfs --options nosuid,nodev,noexec shm /dev/shm
chmod 1777 /dev/shm /run/shm &>/dev/null

echo -e "\nRun The Following Commands:\n"
echo -e "chroot /mnt/gentoo /bin/bash\n"
echo -e "source /etc/profile\nexport PS1=\"(chroot) \${PS1}\"\nsetenforce 0\n"
