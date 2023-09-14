#!/bin/bash

installServer="$1"
locale="$2"
DistroName="$3"
cpuType="$4"


DistroNameLower="$(echo "$DistroName" | tr '[:upper:]' '[:lower:]')"

if [ "$(grep "^hostname=" /etc/conf.d/hostname)" != "" ]; then
  sed -r -i "s/^hostname=\".*\"$/hostname=\"$DistroNameLower\"/m" /etc/conf.d/hostname
else
  echo "hostname=\"$DistroNameLower\"" >> /etc/conf.d/hostname
fi

sed -r -i "s/^(127\.0\.0\.1|::1)(\s*)localhost$/\1\2$DistroNameLower localhost/m" /etc/hosts


emerge --noreplace --quiet net-misc/netifrc


#todo: automate dhcp config
# ifconfig
# nano /etc/conf.d/net
# config_wlp0s20f3="dhcp"

# auto start network on boot
# ln -s /etc/init.d/net.lo /etc/init.d/net.wlp0s20f3
# rc-update add net.wlp0s20f3 default


# set root passwd
# nano /etc/security/passwdqc.conf
sed -r -i 's/^min=.*$/min=4,4,4,4,4/m' /etc/security/passwdqc.conf
sed -r -i 's/^max=.*$/max=72/m' /etc/security/passwdqc.conf
sed -r -i 's/^passphrase=.*$/passphrase=3/m' /etc/security/passwdqc.conf
sed -r -i 's/^match=.*$/match=4/m' /etc/security/passwdqc.conf
sed -r -i 's/^similar=.*$/similar=permit/m' /etc/security/passwdqc.conf
sed -r -i 's/^enforce=.*$/enforce=everyone/m' /etc/security/passwdqc.conf
sed -r -i 's/^retry=.*$/retry=3/m' /etc/security/passwdqc.conf
# passwd


#todo: ask seperately for a keyboard layout (instead of guessing based on locale)
nano /etc/conf.d/keymaps
keymap="$(echo "$locale" | sed -E 's/^[a-z]+_([A-Z]+)$/\1/' | tr '[:upper:]' '[:lower:]')"
if [ "$keymap" != "" ]; then
  sed -r -i "s/^keymap=\".*\"$/keymap=\"$keymap\"/m" /etc/conf.d/keymaps
fi

#todo: detect if duel booting windows
# may need to change from UTC to local if clock is off (I think I remember heaing somewhere, changing this may help if duel booting windows)
# nano /etc/conf.d/hwclock


# install logger
emerge --quiet app-admin/sysklogd
rc-update add sysklogd default

# install cron
emerge --quiet sys-process/cronie
rc-update add cronie default

# file indexing
emerge --quiet sys-apps/mlocate

# bash completion
emerge --quiet app-shells/bash-completion

if [ "$installServer" = "y" ]; then
  rc-update add sshd default

  # uncomment section: SERIAL CONSOLES
  # nano /etc/inittab
  rm -f inittab.tmp
  mode="0"
  while read -r line; do
    if [ "$line" = "# SERIAL CONSOLES" ]; then
      mode="1"
      echo "$line" >> inittab.tmp
    elif [ "$mode" = "1" ]; then
      if [ "$line" = "" ]; then
        mode="0"
        echo "$line" >> inittab.tmp
      else
        echo "$(echo "$line" | sed -E 's/^#\s*//')" >> inittab.tmp
      fi
    else
      echo "$line" >> inittab.tmp
    fi
  done < /etc/inittab
  unset mode
  mv inittab.tmp /etc/inittab
fi

# install dhcp client and wireless tools
emerge --quiet net-misc/dhcpcd
emerge --quiet net-wireless/iw net-wireless/wpa_supplicant


# install grub
emerge --newuse --deep --quiet sys-boot/grub
emerge --newuse --quiet sys-boot/os-prober

grub-install --target="$cpuType-efi" --efi-directory=/boot --bootloader-id="$DistroName" --removable
# grub-install --target="$cpuType-efi" --efi-directory=/boot --bootloader-id="$DistroName" --removable --uefi-secure-boot


#todo: for dule booting windows
echo '' >> /etc/default/grub
echo '# More Options' >> /etc/default/grub
echo 'GRUB_DISABLE_OS_PROBER=false' >> /etc/default/grub

grubTimeout="5"
if [ "$(grep "^#*GRUB_TIMEOUT=" /etc/default/grub)" != "" ]; then
  sed -r -i "s/^#*GRUB_TIMEOUT=.*$/GRUB_TIMEOUT=$grubTimeout/m" /etc/default/grub
else
  echo "GRUB_TIMEOUT=$grubTimeout" >> /etc/default/grub
fi

if [ "$(grep "^#*GRUB_TIMEOUT_STYLE=" /etc/default/grub)" != "" ]; then
  sed -r -i 's/^#*GRUB_TIMEOUT_STYLE=.*$/GRUB_TIMEOUT_STYLE=menu/m' /etc/default/grub
else
  echo 'GRUB_TIMEOUT_STYLE=menu' >> /etc/default/grub
fi

#todo: look at "GRUB_DISTRIBUTOR" option, and see if Gentoo can be replaced with a custom name


grub-mkconfig -o /boot/grub/grub.cfg
