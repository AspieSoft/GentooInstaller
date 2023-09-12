#!/bin/bash

installDisk="$1"
gameSize="$2"
winPart="$3"
DistroName="$4"


# set defaults
installRootSize="16000"
# installVarSize="16000"
installRamSize="0"
installGameSize="0"
installWinSize="0"

# get ram mb
memTotal="$(grep MemTotal /proc/meminfo | sed -E 's/^.*?\s+([0-9]+).*$/\1/')"
memTotal="$((memTotal / 1000))"

# get disk mb
maxDiskSize="$(lsblk -nlo size /dev/mmcblk0 | head -n1 | sed -E 's/\.[0-9]+([MG])$/\1/')"
if [[ "$maxDiskSize" =~ ^[0-9]+[MG]$ ]]; then
  if [[ "$maxDiskSize" =~ G$ ]]; then
    maxDiskSize="$(echo "$maxDiskSize" | sed -e 's/[MG]$//')"
    maxDiskSize=$((maxDiskSize * 1000))
  else
    maxDiskSize="$(echo "$maxDiskSize" | sed -e 's/[MG]$//')"
  fi

  maxDiskSize="$((maxDiskSize - 260))"

  if [ "$maxDiskSize" -ge "512000" ]; then
    installRootSize="32000"
  elif [ "$maxDiskSize" -le "128000" ]; then
    installRootSize="8000"
  fi

  installRamSize="$((16000 - memTotal))"
  if [ "$installRamSize" -lt "2000" -a "$installRamSize" -gt "0" ]; then
    installRamSize="2000"
  elif [ "$installRamSize" -le "0" ]; then
    installRamSize="0"
  fi

  if [ "$maxDiskSize" -le "16000" ]; then
    installRamSize="0"
  elif [ "$maxDiskSize" -le "64000" -a "$installRamSize" -gt "2000" ]; then
    installRamSize="2000"
  elif [ "$maxDiskSize" -le "128000" -a "$installRamSize" -gt "4000" ]; then
    installRamSize="4000"
  elif [ "$maxDiskSize" -le "256000" -a "$installRamSize" -gt "8000" ]; then
    installRamSize="8000"
  elif [ "$installRamSize" -gt "16000" ]; then
    installRamSize="16000"
  fi

  maxDiskSize=$((maxDiskSize - installRootSize - installRamSize))

  maxDiskSize=$((maxDiskSize - 128000))

  if [ "$maxDiskSize" -lt "128000" ]; then
    maxDiskSize="0"
  fi

  if ! [ "$maxDiskSize" = "0" ]; then
    if [ "$gameSize" != "" ]; then
      installGameSize="$gameSize"
      if [[ "$installGameSize" =~ ^[0-9]+[MG]?$ ]]; then
        if ! [[ "$installGameSize" =~ [MG]$ ]]; then
          installGameSize="${installGameSize}G"
        fi

        if [[ "$installGameSize" =~ G$ ]]; then
          installGameSize="$(echo "$installGameSize" | sed -e 's/[MG]$//')"
          installGameSize=$((installGameSize * 1000))
        else
          installGameSize="$(echo "$installGameSize" | sed -e 's/[MG]$//')"
        fi

        if [ "$installGameSize" -le "$maxDiskSize" ]; then
          maxDiskSize=$((maxDiskSize - installGameSize))
        elif [ "$winPart" = "y" ]; then
          installGameSize="$((maxDiskSize - 16000))"
          if [ "$installGameSize" -ge "16000" ]; then
            maxDiskSize=$((maxDiskSize - installGameSize))
          else
            installGameSize="$((maxDiskSize - 8000))"
            if [ "$installGameSize" -ge "8000" ]; then
              maxDiskSize=$((maxDiskSize - installGameSize))
            else
              installGameSize="0"
            fi
          fi
        else
          installGameSize="$maxDiskSize"
        fi
      fi
    else
      while true; do
        echo
        echo "Max Size: $((maxDiskSize / 1000))GB"

        echo
        echo "(leave blank for no game partition)"
        read -p "Game Partition Size (GB): " installGameSize
        echo

        if [ "$installGameSize" = "" ]; then
          break
        fi

        if [[ "$installGameSize" =~ ^[0-9]+[MG]?$ ]]; then
          if ! [[ "$installGameSize" =~ [MG]$ ]]; then
            installGameSize="${installGameSize}G"
          fi

          if [[ "$installGameSize" =~ G$ ]]; then
            installGameSize="$(echo "$installGameSize" | sed -e 's/[MG]$//')"
            installGameSize=$((installGameSize * 1000))
          else
            installGameSize="$(echo "$installGameSize" | sed -e 's/[MG]$//')"
          fi

          if [ "$installGameSize" -le "$maxDiskSize" ]; then
            maxDiskSize=$((maxDiskSize - installGameSize))
            break
          else
            echo "error: not enough disk space (maximum: $((maxDiskSize / 1000))GB)"
            continue
          fi
        fi

        echo "error: Invalid Input (must be a valid number '^[0-9]+[MG]?$' | blank to skip)"
      done
    fi

  fi

  if [ "$maxDiskSize" -ge "64000" ]; then
    if [ "$winPart" != "" ]; then
      includeWinPart="$winPart"
    else
      read -p "Would you like a windows partition? (Y/n): " includeWinPart
    fi

    if ! [ "$includeWinPart" = "n" -o "$includeWinPart" = "N" -o "$includeWinPart" = "no" -o "$includeWinPart" = "No" -o "$includeWinPart" = "NO" -o "$includeWinPart" = "false" -o "$includeWinPart" = "FALSE" -o "$includeWinPart" = "0" ]; then
      if [ "$maxDiskSize" -ge "1024000" ]; then
        installWinSize="256000"
      elif [ "$maxDiskSize" -ge "512000" ]; then
        installWinSize="128000"
      elif [ "$maxDiskSize" -ge "256000" ]; then
        installWinSize="64000"
      elif [ "$maxDiskSize" -ge "128000" ]; then
        installWinSize="32000"
      elif [ "$maxDiskSize" -ge "64000" ]; then
        installWinSize="16000"
      fi
    fi
  fi
fi

echo


echo "wiping disk..."
wipefs --all "/dev/$installDisk" &>/dev/null
mkfs.ext4 "/dev/${installDisk}" &>/dev/null
wipefs --all "/dev/$installDisk" &>/dev/null

echo "seting up disk..."
./bin/go/disk/disk "$PWD/bin/scripts/core/disk.yml" --rootSize="$installRootSize" --ramSize="$installRamSize" --gameSize="$installGameSize" --winSize="$installWinSize" | parted -a optimal "/dev/$installDisk" &>/dev/null

sleep 5 #fix: allow parted to finished running

partList="$(lsblk -nlo name,partlabel "/dev/$installDisk" 2>/dev/null)"
if [ "$partList" = "" ]; then
  echo "error: failed to find any partitions on disk!"
  exit
fi

#fix: using a file to escape subshell created by while loop
rm -f ./parteval.tmp
touch ./parteval.tmp

echo "$partList" | while read -r line; do
  label="$(echo $line | sed -E 's/^[A-Za-z0-9]*\s*//')"
  if ! [ "$label" = "" ]; then
    part="$(echo "$line" | sed -E 's/\s+.*$//')"
    echo "diskpart_$label=\"$part\"" | tee -a ./parteval.tmp &>/dev/null
  fi
done

sleep 5

eval $(cat ./parteval.tmp)
rm -f ./parteval.tmp

echo "creating partition filesystems..."

if ! [ "$diskpart_boot" = "" ]; then
  echo " - creating boot partition..."
  mkfs.vfat "/dev/$diskpart_boot" &>/dev/null
  fatlabel "/dev/$diskpart_boot" "$(echo "$DistroName" | tr '[:lower:]' '[:upper:]')"
else
  echo "error: faied to find boot partition!"
  exit
fi

if ! [ "$diskpart_root" = "" ]; then
  echo " - creating root partition..."
  mkfs.xfs -q "/dev/$diskpart_root"
else
  echo "error: faied to find root partition!"
  exit
fi

if ! [ "$diskpart_var" = "" ]; then
  echo " - creating var/home partition..."
  mkfs.btrfs -f "/dev/$diskpart_var" -q
fi

if ! [ "$diskpart_games" = "" ]; then
  echo " - creating games partition..."
  mkfs.ext4 "/dev/$diskpart_games" &>/dev/null
fi

if ! [ "$diskpart_windows" = "" ]; then
  echo " - creating windows (fat32) partition..."
  mkfs.vfat -F 32 "/dev/$diskpart_windows" &>/dev/null
fi

#todo: lookup encrypted partitions

if ! [ "$diskpart_swap" = "" ]; then
  echo " - creating linux swap partition..."
  mkswap "/dev/$diskpart_swap" -q
fi


# mount partitions
mkdir -p /mnt/gentoo
sudo chmod 555 /mnt/gentoo
mount "/dev/$diskpart_root" /mnt/gentoo
mkdir /mnt/gentoo/boot
sudo chmod 555 /mnt/gentoo/boot
mount "/dev/$diskpart_boot" /mnt/gentoo/boot

if ! [ "$diskpart_var" = "" ]; then
  mkdir /mnt/gentoo/var
  sudo chmod 755 /mnt/gentoo/var
  mount "/dev/$diskpart_var" /mnt/gentoo/var
fi

if ! [ "$diskpart_games" = "" ]; then
  mkdir /mnt/gentoo/games
  sudo chmod 755 /mnt/gentoo/games
  mount "/dev/$diskpart_games" /mnt/gentoo/games
fi

if ! [ "$diskpart_windows" = "" ]; then
  mkdir /mnt/gentoo/windows
  sudo chmod 755 /mnt/gentoo/windows
  mount "/dev/$diskpart_windows" /mnt/gentoo/windows
fi

if ! [ "$diskpart_swap" = "" ]; then
  swapon "/dev/$diskpart_swap"
fi

sleep 5
