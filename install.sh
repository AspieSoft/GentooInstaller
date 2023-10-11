#!/bin/bash

cd $(dirname "$0")
dir="$PWD"

if [ "$(ls "/mnt/gentoo/gentoo-installer/running.resume" 2>/dev/null)" != "" -a "$(cat "/mnt/gentoo/gentoo-installer/running.resume" 2>/dev/null)" = "Gentoo Installer: running emerge @world" ]; then
  locale="$(cat "/mnt/gentoo/gentoo-installer/var/locale")"
  #todo: set script to use $locale as language
  
  source "$dir/bin/scripts/core/chroot.sh" "$dir" "--resume"
  source "$dir/bin/scripts/core/cleanup.sh"
  exit
fi

installServer=""
locale=""
installDisk=""
gameSize=""
winPart=""

for arg in $@; do
  if [ "$arg" = "-s" -o "$arg" = "--server" ]; then
    installServer="y"
  elif [[ "$arg" =~ "--lang="* ]] || [[ "$arg" =~ "--locale="* ]]; then
    locale="$(echo "$arg" | sed -E 's/^--(lang|locale)=//')"
    if [ "$locale" = "" ]; then
      locale="0"
    fi
  elif [ "$arg" = "-L" -o "$arg" = "--lang" -o "$arg" = "--locale" ]; then
    locale="0"
  elif [[ "$arg" =~ "--disk="* ]]; then
    installDisk="$(echo "$arg" | sed -E 's/^--disk=//')"
  elif [[ "$arg" =~ "--game="* ]]; then
    gameSize="$(echo "$arg" | sed -E 's/^--game=//')"
  elif [ "$arg" = "-G" -o "$arg" = "--game" ]; then
    gameSize="120"
  elif [[ "$arg" =~ "--win="* ]] || [[ "$arg" =~ "--windows="* ]]; then
    winPart="$(echo "$arg" | sed -E 's/^--(win|windows)=//')"
    if [ "$winPart" = "y" -o "$winPart" = "Y" -o "$winPart" = "yes" -o "$winPart" = "Yes" -o "$winPart" = "YES" -o "$winPart" = "true" -o "$winPart" = "TRUE" -o "$winPart" = "1" ]; then
      winPart="y"
    else
      winPart="n"
    fi
  elif [ "$arg" = "-W" -o "$arg" = "--win" -o "$arg" = "--windows" ]; then
    winPart="y"
  fi
done


if [ "$locale" = "0" ]; then
  locale="$(locale | grep "LANG" | sed -E 's/^.*?=(.*?)\..*$/\1/')"
else
  defaultLocale="$(locale | grep "LANG" | sed -E 's/^.*?=(.*?)\..*$/\1/')"

  if [ "$locale" = "" ]; then
    echo -e "[0] $defaultLocale"
  fi

  listN="0"
  i="0"
  for l in $(ls /usr/share/locale); do
    if [ "$(echo "$l" | sed -E 's/^[a-z]+(_[A-Z]+|)$//')" != "" ]; then
      continue
    fi

    listN="$((listN + 1))"
    i="$((i + 1))"
    localeList[$listN]="$l"

    if [ "$locale" != "" ]; then
      continue
    fi

    echo -e "[$listN] $l                              "

    if [ "$i" -ge 10 ]; then
      i="0"
      echo
      echo "(Leave Blank To See More...)"
      echo
      read -p "Choose Locale: " locale
      if [ "$locale" != "" ]; then
        continue
      fi

      echo -e "\e[4A\e[K----------                    "
    fi
  done
  unset listN
  unset i

  if [ "$locale" == "" ]; then
    read -p "Choose Locale: " locale
  fi

  if [ "$locale" == "" -o "$locale" == "0" ]; then
    locale="$defaultLocale"
  elif [ "$(echo "$locale" | sed -E 's/^[0-9]*$//')" = "" ]; then
    locale="${localeList[$locale]}"
  fi

  lList="$(echo "${localeList[@]}" | sed -E 's/ /\n/g')"
  unset localeList
  l="$(echo -e "$lList" | grep "^$locale" | head -n1)"
  if [ "$l" = "" ]; then
    l="$(echo -e "$lList" | grep -i "^$locale" | head -n1)"
  fi
  if [ "$l" = "" ]; then
    l="$defaultLocale"
  fi
  locale="$l"
  unset l
  unset defaultLocale
  unset lList
fi


#todo: set script to use $locale as language
# may also try adding a golang gui window to handle translations
# a gui may also add an easier and nicer looking Installation process


timezone="$(curl -sL "https://ipapi.co/timezone")"
continent="$(cat "$dir/bin/geo_continent.yml" | grep "$(curl -sL "https://ipapi.co/continent_code")" | sed -E 's/^[A-Z]*:\s*//')"

DistroName="$(cat "$dir/config/DistroName.conf")"

if [ "$installDisk" = "" ]; then
  # ask for disk to wipe for install
  while true; do
    echo
    lsblk -e7 -o name,size,label,fstype,mountpoint,uuid -T

    echo
    read -p "Choose A Disk To Wipe For The Install: " installDisk
    echo

    installDisk="$(lsblk -lino name "/dev/$installDisk" 2>/dev/null | head -n1 | sed -E 's/^([A-Za-z0-9]+).*/\1/')"
    if ! [ "$installDisk" = "" ]; then
      echo "Are you sure you would like to wipe the following disk?"
      echo "$installDisk"
      echo "This Cannot Be Undone"
      read -p "(yes|No): " installDiskConfirm
      if [ "$installDiskConfirm" = "yes" ]; then
        break
      else
        installDisk=""
        echo "wipe canceled!"
        continue
      fi
    fi

    echo "error: invalid disk!"
  done
else
  installDisk="$(lsblk -lino name "/dev/$installDisk" 2>/dev/null | head -n1 | sed -E 's/^([A-Za-z0-9]+).*/\1/')"
  if [ "$installDisk" = "" ]; then
    echo "error: invalid disk!"
    exit
  fi
fi

# auto detect cpu type or ask user
if ! [ "$(lscpu | grep "[Ii]ntel")" = "" ]; then
  if [ "$(lscpu | grep "64-bit")" ]; then
    cpuType="amd64"
    realCPUType="x86_64"
  else
    cpuType="x86"
    cpuType2="i686"
    realCPUType="i686"
  fi
elif ! [ "$(lscpu | grep "[Aa][Mm][Dd]")" = "" ]; then
  cpuType="amd64"
  realCPUType="amd64"
elif ! [ "$(lscpu | grep "[Aa][Rr][Mm]")" = "" ]; then
  if [ "$(lscpu | grep "64-bit")" ]; then
    cpuType="arm64"
    realCPUType="arm64"
  else
    cpuType="arm"
    realCPUType="arm"
  fi
else
  read -p "Select CPU Type: " cpuType
  if [ "$cpuType" = "" ] || ! [[ "$cpuType" =~ ^[A-Za-z0-9]*$ ]]; then
    echo "Error: Invalid CPU Type!"
    echo "example: amd64, arm64, arm, x86_64, x86, i686"
    echo "alias: intel64, intel"
    exit
  fi
fi

if [ "$cpuType2" = "" ]; then
  if [ "$cpuType" = "intel64" -o "$cpuType" = "x86_64" ]; then
    cpuType="amd64"
    cpuType2="i686"
    realCPUType="x86_64"
  elif [ "$cpuType" = "intel" -o "$cpuType" = "x86" -o "$cpuType" = "i686" ]; then
    cpuType="x86"
    cpuType2="i686"
    realCPUType="i686"
  else
    cpuType2="$cpuType"
  fi
fi

if [ "$realCPUType" = "" ]; then
  realCPUType="$cpuType2"
fi


# get tarball stage3 file
stage3File=""
installID=""
mode="0"
for tarball in $(cat "$dir/config/tarball.conf"); do
  if [ "$tarball" = "[Server]" -o "$tarball" = "[server]" ]; then
    mode="s"
    continue
  elif [ "$tarball" = "[Desktop]" -o "$tarball" = "[desktop]" ]; then
    mode="d"
    continue
  fi

  if [ "$mode" = "s" -a "$installServer" != "y" ]; then
    continue
  fi

  stage3File="$tarball"
  installID="$(curl -sL "https://distfiles.gentoo.org/releases/${cpuType}/autobuilds/current-stage3-${cpuType2}-${stage3File}" | grep "stage3-${cpuType2}-${stage3File}-[A-Za-z0-9]*\.tar\." | head -n1 | sed -E "s/^.*?stage3-${cpuType2}-${stage3File}-([A-Za-z0-9]*)\.tar\.([A-Za-z0-9]*).*$/\1.tar.\2/")"
  if [ "$installID" != "" ]; then
    break
  fi
done

# verify success
if [ "$installID" = "" ] || ! [[ "$installID" =~ ^[A-Za-z0-9\.]*$ ]]; then
  echo "Error: Failed To Find Installation File!"
  if ! [ "$cpuType" = "amd64" -o "$cpuType" = "arm64" -o "$cpuType" = "x86" ]; then
    echo "cpu type example: amd64, arm64"
  fi
  exit
fi

echo "Selected Tarball: $stage3File"


source "$dir/bin/scripts/core/disk.sh" "$installDisk" "$gameSize" "$winPart" "$DistroName"

source "$dir/bin/scripts/core/tarball.sh" "$dir" "$cpuType" "$cpuType2" "$stage3File" "$installID"

source "$dir/bin/scripts/core/setup.sh" "$dir" "$installServer"

source "$dir/bin/scripts/core/chroot.sh" "$dir" "$installDisk" "$installServer" "$timezone" "$continent" "$locale" "$DistroName" "$realCPUType"

source "$dir/bin/scripts/core/cleanup.sh"
