#!/bin/bash

cd /mnt/gentoo
dir="$1"

if ! [ "$2" = "--resume" ]; then
  installDisk="$2"
  installServer="$3"
  timezone="$4"
  continent="$5"
  locale="$6"
  DistroName="$7"
  cpuType="$8"

  # copy nececary scripts
  mkdir -p /mnt/gentoo/gentoo-installer/scripts
  cp -r "$dir/bin/scripts/core/chroot" "/mnt/gentoo/gentoo-installer/scripts/core"
  cp -f "$dir/bin/go/emerge-world/emerge-world" "/mnt/gentoo/gentoo-installer/scripts/core/emerge-world"

  rsync -av --progress "$dir/bin/scripts/dist" "/mnt/gentoo/gentoo-installer/scripts" --exclude="server" --exclude="desktop"
  if [ "$installServer" = "y" ]; then
    cp -rf "$dir/bin/scripts/dist/server/"* /mnt/gentoo/gentoo-installer/scripts/dist
  else
    cp -rf "$dir/bin/scripts/dist/desktop/"* /mnt/gentoo/gentoo-installer/scripts/dist
  fi

  cp -r "$dir/bin/assets" "/mnt/gentoo/gentoo-installer/assets"

  mkdir -p /mnt/gentoo/gentoo-installer/var
  echo "$timezone" > /mnt/gentoo/gentoo-installer/var/timezone
  echo "$continent" > /mnt/gentoo/gentoo-installer/var/continent
  echo "$locale" > /mnt/gentoo/gentoo-installer/var/locale
  echo "$installDisk" > /mnt/gentoo/gentoo-installer/var/installDisk
  echo "$installServer" > /mnt/gentoo/gentoo-installer/var/installServer
  echo "$DistroName" > /mnt/gentoo/gentoo-installer/var/DistroName
  echo "$cpuType" > /mnt/gentoo/gentoo-installer/var/cpuType
fi

# mounting necesay filesystems
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

# enter new environment
chroot /mnt/gentoo /bin/bash /gentoo-installer/scripts/core/setup.sh

# after chroot exits
cd
umount -l /mnt/gentoo/dev{/shm,/pts,}
umount -R /mnt/gentoo
