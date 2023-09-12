#!/bin/bash

dir="$1"
installServer="$2"

# todo: get safe flags for current cpu (https://wiki.gentoo.org/wiki/Safe_CFLAGS)
# also read this step carefully (https://wiki.gentoo.org/wiki/Handbook:AMD64/Installation/Stage)
# nano /mnt/gentoo/etc/portage/make.conf

vendorInfo="$(grep -m1 -A3 "vendor_id" /proc/cpuinfo)"
vendorID="$(echo "$vendorInfo" | grep "^vendor_id" | head -n1 | sed -E 's/[A-Za-z0-9_ -]*\s*:\s*//')"
cpuFamily="$(echo "$vendorInfo" | grep "^cpu family" | head -n1 | sed -E 's/[A-Za-z0-9_ -]*\s*:\s*//')"
modelID="$(echo "$vendorInfo" | grep "^model" | head -n1 | sed -E 's/[A-Za-z0-9_ -]*\s*:\s*//')"

modelRegex="[Ii]ntel|AMD|[Aa]md"
modelName="$(echo "$vendorInfo" | grep "^model name" | head -n1 | sed -E 's/[A-Za-z0-9_ -]*\s*:\s*//' | sed -E "s/^.*?($modelRegex)(\\([Rr]\\)|)\\s*([A-Za-z0-9_-]*).*$/\\3/")"

#todo: finish adding flags to cpu_flags.yml (https://wiki.gentoo.org/wiki/Safe_CFLAGS#Intel)
# also double check this file was handled correctly
# cpuFlags="$(grep "^$vendorID $modelName $cpuFamily $modelID" "$dir/bin/cpu_flags.yml" | sed -E 's/^[A-Za-z0-9_ ]*:\s*//')"

#todo: consider adding user option to always use native
if [ "$cpuFlags" = "" ]; then
  cpuFlags="-march=native"
fi

sed -r -i "s/^COMMON_FLAGS=\"(.*)\"$/COMMON_FLAGS=\"$cpuFlags \\1\"/m" /mnt/gentoo/etc/portage/make.conf


cpuCors="$(lscpu | grep "^CPU(s):" | sed -E 's/^CPU\(s\):\s*//')"
cpuCoreThreads="$(lscpu | grep "^Thread(s) per core:" | sed -E 's/^Thread\(s\) per core:\s*//')"
cpuCors=$((cpuCors / cpuCoreThreads))
unset cpuCoreThreads

memTotal="$(grep MemTotal /proc/meminfo | sed -E 's/^.*?\s+([0-9]+).*$/\1/')"
memTotal="$((memTotal / 1000))"
if [ "$(echo "$memTotal" | sed -E 's/^[0-9]*([0-9]{3})$/\1/')" -lt "500" ]; then
  memTotal="$((memTotal / 1000))"
else
  memTotal="$((memTotal / 1000 + 1))"
fi
memTotal="$((memTotal / 2))"

if [ "$memTotal" -lt "$cpuCors" ]; then
  cpuCors="$memTotal"
fi
unset memTotal

if [ "$(grep "^MAKEOPT=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i "s/^MAKEOPT=\"(.*)\"$/MAKEOPT=\"\\1 -j$cpuCors\"/m" /mnt/gentoo/etc/portage/make.conf
elif [ "$(grep "^FFLAGS=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i "s/^(FFLAGS=\".*\")$/\\1\nMAKEOPT=\"-j$cpuCors\"/m" /mnt/gentoo/etc/portage/make.conf
elif [ "$(grep "^FCFLAGS=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i "s/^(FCFLAGS=\".*\")$/\\1\nMAKEOPT=\"-j$cpuCors\"/m" /mnt/gentoo/etc/portage/make.conf
elif [ "$(grep "^COMMON_FLAGS=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i "s/^(COMMON_FLAGS=\".*\")$/\\1\nMAKEOPT=\"-j$cpuCors\"/m" /mnt/gentoo/etc/portage/make.conf
else
  echo -e "MAKEOPT=\"-j$cpuCors\"\n" >> /mnt/gentoo/etc/portage/make.conf
fi
unset cpuCors

if [ "$(grep "^ACCEPT_LICENSE=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i 's/^ACCEPT_LICENSE="(.*)"$/ACCEPT_LICENSE="*"/m' /mnt/gentoo/etc/portage/make.conf
else
  sed -r -i 's/^(MAKEOPT=".*")$/\1\nACCEPT_LICENSE="*"/m' /mnt/gentoo/etc/portage/make.conf
fi

# server | hardened selinux
useFlags="xfsprogs btrfs-progs device-mapper efiemu mount nls sdl truetype python -systemd -qtwebenging -webenging"

# desktop | basic | hardened
if [ "$installServer" != "y" ]; then
  useFlags="dbus png jpeg webp xfsprogs gui gtk X libnotify -qt5 joystick opengl sound video bluetooth network multimedia printsupport location $useFlags"
  #todo: try adding "-netboot" to USE flags, and see if that speeds up the "emerge @world" step
  # -netboot
fi

if [ "$(grep "^USE=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i "s/^USE=\"(.*)\"$/USE=\"\\1 $useFlags\"/m" /mnt/gentoo/etc/portage/make.conf
else
  sed -r -i "s/^(ACCEPT_LICENSE=\".*\")$/\\1\nUSE=\"$useFlags\"/m" /mnt/gentoo/etc/portage/make.conf
fi
unset useFlags

grupPlatforms="emu efi-32 efi-64 pc"
if [ "$(grep "^GRUB_PLATFORMS=" /mnt/gentoo/etc/portage/make.conf)" != "" ]; then
  sed -r -i "s/^GRUB_PLATFORMS=\"(.*)\"$/GRUB_PLATFORMS=\"\\1 $grupPlatforms\"/m" /mnt/gentoo/etc/portage/make.conf
else
  sed -r -i "s/^(USE=\".*\")$/\\1\nGRUB_PLATFORMS=\"$grupPlatforms\"/m" /mnt/gentoo/etc/portage/make.conf
fi
unset grupPlatforms


# copy dns info
cp --dereference /etc/resolv.conf /mnt/gentoo/etc/
