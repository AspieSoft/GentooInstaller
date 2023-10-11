#!/bin/bash

source /etc/profile
export PS1="(chroot) ${PS1}"

dir="/gentoo-installer"
timezone="$(cat "$dir/var/timezone")"
continent="$(cat "$dir/var/continent")"
locale="$(cat "$dir/var/locale")"
installDisk="$(cat "$dir/var/installDisk")"
installServer="$(cat "$dir/var/installServer")"
DistroName="$(cat "$dir/var/DistroName")"
cpuType="$(cat "$dir/var/cpuType")"


if ! [ "$(ls "$dir/running.resume" 2>/dev/null)" != "" -a "$(cat "$dir/running.resume" 2>/dev/null)" = "Gentoo Installer: running emerge @world" ]; then
  # move and link directories
  # this allows some root directories to be moved within the btrfs partition, instead of the xfs partition
  mv /home /var/home
  ln -s /var/home /home
  mv /root /var/roothome
  ln -s /var/roothome /root
  mv /usr/share /var/usrshare
  ln -s /var/usrshare /usr/share


  # install theme assets
  mkdir -p "/usr/share/themes"
  mkdir -p "/usr/share/icons"
  mkdir -p "/usr/share/sounds"
  mkdir -p "/usr/share/backgrounds"
  tar -xzf "$dir/assets/theme/themes.tar.gz" -C "/usr/share/themes"
  tar -xzf "$dir/assets/theme/icons.tar.gz" -C "/usr/share/icons"
  tar -xzf "$dir/assets/theme/sounds.tar.gz" -C "/usr/share/sounds"
  tar -xzf "$dir/assets/theme/backgrounds.tar.gz" -C "/usr/share/backgrounds"
  rm -rf $dir/assets/theme


  # install gentoo ebuild
  emerge-webrsync


  # add mirrors
  emerge --quiet app-portage/mirrorselect
  source "$dir/scripts/core/mirrors.sh" "$continent"


  # run emerge sync (and retry up to 3 times to handle possible race condition)
  echo 'running "emerge --sync" command...'
  retries="3"
  emerge --sync &>sync_log.tmp
  # consider adding "--quiet" flag to emerge commands
  while [ "$(cat sync_log.tmp | grep "returned code = 0$")" = "" ]; do
    echo 'command failed!'
    retries="$((retries-1))"
    if [ "$retries" -le "0" ]; then
      break
    fi

    echo "retries left: $retries"
    echo 'trying "emerge --sync" again...'

    echo "" > sync_log.tmp
    emerge --sync &>sync_log.tmp
  done
  rm -f sync_log.tmp

  # install rust-bin binary to prevent the next step from overheating the CPU
  emerge --quiet dev-lang/rust-bin

  # update @world set
  echo 'Gentoo Installer: running emerge @world' > "$dir/running.resume"
  # emerge --update --deep --newuse --quiet @world
  "$dir/scripts/core/emerge-world" # golang script to run command and track cpu temp, and pause as needed
  rm -f "$dir/running.resume"
else
  # resume update @world set
  # emerge --resume --quiet
  "$dir/scripts/core/emerge-world" --resume # golang script to run command and track cpu temp, and pause as needed
  rm -f "$dir/running.resume"
fi

sleep 5

# install cpuflags
emerge --quiet app-portage/cpuid2cpuflags
echo "*/* $(cpuid2cpuflags)" > /etc/portage/package.use/00cpu-flags

if [ "$installServer" != "y" ]; then
  emerge --quiet x11-base/xorg-server
fi


source "$dir/scripts/core/timezone.sh" "$timezone" "$locale"


# reload environment
env-update && source /etc/profile && export PS1="(chroot) ${PS1}"


# install linux firmware
emerge --quiet sys-kernel/linux-firmware

# install kernel
emerge --quiet sys-kernel/gentoo-kernel-bin

# install filesystems
emerge --quiet sys-fs/e2fsprogs
emerge --quiet sys-fs/dosfstools
emerge --quiet sys-fs/xfsprogs
emerge --quiet sys-fs/btrfs-progs
emerge --quiet sys-fs/f2fs-tools
emerge --quiet sys-fs/ntfs3g

# add partitoons to fstab
lsblk "/dev/$installDisk" -lino partlabel,UUID | while read -r line; do
  if [[ "$line" =~ "boot"* ]]; then
    uuid="$(echo "$line" | sed -E 's/^boot\s*//')"
    echo "UUID=$uuid    /boot    vfat    noatime    0 2" >> /etc/fstab
  elif [[ "$line" =~ "swap"* ]]; then
    uuid="$(echo "$line" | sed -E 's/^swap\s*//')"
    echo "UUID=$uuid    none    swap    sw    0 0" >> /etc/fstab
  elif [[ "$line" =~ "root"* ]]; then
    uuid="$(echo "$line" | sed -E 's/^root\s*//')"
    echo "UUID=$uuid    /    xfs    noatime    0 1" >> /etc/fstab
  elif [[ "$line" =~ "var"* ]]; then
    uuid="$(echo "$line" | sed -E 's/^var\s*//')"
    echo "UUID=$uuid    /var    btrfs    noatime    0 2" >> /etc/fstab
  elif [[ "$line" =~ "games"* ]]; then
    uuid="$(echo "$line" | sed -E 's/^games\s*//')"
    echo "UUID=$uuid    /games    ext4    noatime    0 2" >> /etc/fstab
  elif [[ "$line" =~ "windows"* ]]; then
    uuid="$(echo "$line" | sed -E 's/^windows\s*//')"
    echo "UUID=$uuid    /windows    fat32    noatime    0 2" >> /etc/fstab
  fi
done

source "$dir/scripts/core/system.sh" "$installServer" "$locale" "$DistroName" "$cpuType"

# cleanup
emerge --depclean --quiet

#todo: setup secure boot
# source "$dir/scripts/core/secureboot.sh" "$DistroName"

# run distro install
source "$dir/scripts/dist/run.sh" "$dir"

# cleanup
emerge --depclean --quiet

# cleanup and exit chroot
rm -rf "$dir"
exit
