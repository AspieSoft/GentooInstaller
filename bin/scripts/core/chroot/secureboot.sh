#!/bin/bash

DistroName="$1"
DistroNameLower="$(echo "$DistroName" | tr '[:upper:]' '[:lower:]')"



#*Newest

# install tools
emerge --quiet app-crypt/efitools
emerge --quiet app-crypt/sbsigntools
emerge --quiet dev-libs/openssl

emerge --quiet sys-boot/mokutil
emerge --quiet sys-boot/shim

emerge --quiet sys-boot/refind


# create keys dir
mkdir -m 0700 /boot/efikeys
cd /boot/efikeys

# backup old keys
efi-readvar -v PK -o old_PK.esl
efi-readvar -v KEK -o old_KEK.esl
efi-readvar -v db -o old_db.esl
efi-readvar -v dbx -o old_dbx.esl

# generate new keys
openssl req -new -x509 -newkey rsa:2048 -subj "/emailAddress=web@aspiesoft.com/CN=empoleos platform key/O=AspieSoft/C=US/" -keyout PK.key -out PK.crt -days 3650 -nodes -sha256
openssl req -new -x509 -newkey rsa:2048 -subj "/emailAddress=web@aspiesoft.com/CN=empoleos key-exchange key/O=AspieSoft/C=US/" -keyout KEK.key -out KEK.crt -days 3650 -nodes -sha256
openssl req -new -x509 -newkey rsa:2048 -subj "/emailAddress=web@aspiesoft.com/CN=empoleos kernel-signing key/O=AspieSoft/C=US/" -keyout db.key -out db.crt -days 3650 -nodes -sha256
chmod -v 400 *.key


# preparing update files
cert-to-efi-sig-list -g "$(uuidgen)" db.crt db.esl
sign-efi-sig-list -a -k KEK.key -c KEK.crt db db.esl db.auth
cert-to-efi-sig-list -g "$(uuidgen)" PK.crt PK.esl
sign-efi-sig-list -k PK.key -c PK.crt PK PK.esl PK.auth
cert-to-efi-sig-list -g "$(uuidgen)" KEK.crt KEK.esl
sign-efi-sig-list -a -k PK.key -c PK.crt KEK KEK.esl KEK.auth

# update efi vars
efi-updatevar -f db.auth db
efi-updatevar -f KEK.auth KEK
efi-updatevar -f PK.auth PK



# add shim
mv /boot/EFI/BOOT/BOOTX64.EFI /boot/EFI/BOOT/grubx64.efi
cp /usr/share/shim/BOOTX64.EFI /boot/EFI/BOOT/BOOTX64.EFI
cp /usr/share/shim/mmx64.efi /boot/EFI/BOOT/mmx64.efi


# sign bootloader
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/EFI/BOOT/BOOTX64.EFI" /boot/EFI/BOOT/BOOTX64.EFI
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/EFI/BOOT/grubx64.efi" /boot/EFI/BOOT/grubx64.efi
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/EFI/BOOT/mmx64.efi" /boot/EFI/BOOT/mmx64.efi


# sign grub
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/grub/x86_64-efi/grub.efi" "/boot/grub/x86_64-efi/grub.efi"
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/grub/x86_64-efi/core.efi" "/boot/grub/x86_64-efi/core.efi"


#?NEW - OLD



# https://www.youtube.com/watch?v=cR9chz06DDk

#todo: lookup using fedoras kernel and see if it works and is already signed

# install tools
emerge --quiet app-crypt/efitools
emerge --quiet app-crypt/sbsigntools
emerge --quiet dev-libs/openssl

# create keys dir
mkdir -m 0700 /boot/efikeys
cd /boot/efikeys

# backup old keys
efi-readvar -v PK -o old_PK.esl
efi-readvar -v KEK -o old_KEK.esl
efi-readvar -v db -o old_db.esl
efi-readvar -v dbx -o old_dbx.esl

# generate new keys
openssl req -new -x509 -newkey rsa:2048 -subj "/emailAddress=web@aspiesoft.com/CN=empoleos platform key/O=AspieSoft/C=US/" -keyout PK.key -out PK.crt -days 3650 -nodes -sha256
openssl req -new -x509 -newkey rsa:2048 -subj "/emailAddress=web@aspiesoft.com/CN=empoleos key-exchange key/O=AspieSoft/C=US/" -keyout KEK.key -out KEK.crt -days 3650 -nodes -sha256
openssl req -new -x509 -newkey rsa:2048 -subj "/emailAddress=web@aspiesoft.com/CN=empoleos kernel-signing key/O=AspieSoft/C=US/" -keyout db.key -out db.crt -days 3650 -nodes -sha256
chmod -v 400 *.key

# preparing update files
cert-to-efi-sig-list -g "$(uuidgen)" db.crt db.esl
sign-efi-sig-list -a -k KEK.key -c KEK.crt db db.esl db.auth
cert-to-efi-sig-list -g "$(uuidgen)" PK.crt PK.esl
sign-efi-sig-list -k PK.key -c PK.crt PK PK.esl PK.auth
cert-to-efi-sig-list -g "$(uuidgen)" KEK.crt KEK.esl
sign-efi-sig-list -a -k PK.key -c PK.crt KEK KEK.esl KEK.auth

# update efi vars
efi-updatevar -f db.auth db
efi-updatevar -f KEK.auth KEK
efi-updatevar -f PK.auth PK


# sign bootloader
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/EFI/BOOT/BOOTX64.EFI" /boot/EFI/BOOT/BOOTX64.EFI


#todo: fix error (grub prohibited by secure boot policy) on boot
# grub-install --uefi-secure-boot
efibootmgr --create --disk /dev/mmcblk0 --part 1 --label "Empoleos" --loader "/boot/EFI/BOOT/BOOTX64.EFI"

echo sys-apps/systemd-utils boot >> /etc/portage/package.use/01-systemd-boot
emerge --oneshot --update --changed-use sys-apps/systemd-utils



# sign grub
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/grub/x86_64-efi/grub.efi" "/boot/grub/x86_64-efi/grub.efi"
sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/grub/x86_64-efi/core.efi" "/boot/grub/x86_64-efi/core.efi"

# sbsign --key /boot/efikeys/db.key --cert /boot/efikeys/db.crt --output="/boot/grub/x86_64-efi/fat.mod" "/boot/grub/x86_64-efi/fat.mod"

MODULES="all_video archelp boot bufio configfile crypto echo efi_gop efi_uga ext2 extcmd  \
fat font fshelp gcry_dsa gcry_rsa gcry_sha1 gcry_sha512 gettext gfxterm linux linuxefi ls \
memdisk minicmd mmap mpi normal part_gpt part_msdos password_pbkdf2 pbkdf2 reboot relocator \
search search_fs_file search_fs_uuid search_label sleep tar terminal verify video_fb"

grub-mkstandalone -d /boot/grub/x86_64-efi -O x86_64-efi --modules "$MODULES" --pubkey gpg.key --output grubx64.efi /boot/grub/grub.cfg=grub.init.cfg /boot/grub/grub.cfg.sig=grub.init.cfg.sig -v

# grub-mkstandalone --format x86_64-efi --modules "$MODULES" -d /boot/grub/x86_64-efi /boot/grub/grub.efi -o grub.efi
grub-mkstandalone --format x86_64-efi /boot/grub/grub.efi -o /boot/grub/grub.efi


# --disable-shim-lock


#!old - newish



# create key
mkdir -m 0700 /boot/efikeys
cd /boot/efikeys
openssl req -new -x509 -newkey rsa:2048 -subj "/CN=empoleos kernel-signing key/" -keyout kernel.key -out kernel.crt -days 3650 -nodes -sha256
chmod -v 400 *.key

# sign kernel
sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/BOOTX64.EFI" /boot/EFI/BOOT/BOOTX64.EFI

openssl x509 -inform pem -in /boot/efikeys/kernel.crt -outform der -out /boot/efikeys/kernel_cert.der




#!OLD



#https://www.youtube.com/watch?v=RVkCIc8CzR8
# emerge sys-boot/shim sys-boot/mokutil

emerge --quiet app-crypt/efitools
emerge --quiet app-crypt/sbsigntools
emerge --quiet dev-libs/openssl
#todo: may need to make a seperate secure boot method for arm
emerge --quiet sys-boot/mokutil
emerge --quiet sys-boot/shim

emerge --quiet sys-boot/refind

# grub-install --target=x86_64-efi --efi-directory=/boot --modules="efi_gop efi_uga ieee1275_fb vbe vga video_bochs video_cirrus lvm ext2 all_video gfxterm gettext gzio part_gpt fat tpm" --bootloader-id=GRUB
# grub-install --efi-directory=/boot --sbat /usr/share/grub/sbat.csv
grub-install --efi-directory=/boot --sbat /boot/grub/grub.cfg

# cp /usr/share/shim/BOOTX64.EFI /boot/EFI/gentoo/BOOTX64.EFI
# cp /usr/share/shim/mmx64.efi /boot/EFI/gentoo/mmx64.efi

# with --removable flag
mv /boot/EFI/BOOT/BOOTX64.EFI /boot/EFI/BOOT/grubx64.efi
cp /usr/share/shim/BOOTX64.EFI /boot/EFI/BOOT/BOOTX64.EFI
cp /usr/share/shim/mmx64.efi /boot/EFI/BOOT/mmx64.efi

# efibootmgr --disk="/dev/mmcblk0" --part="1" --create --label="Empoleos" --loader /EFI/BOOT/BOOTX64.EFI

mkdir -m 0700 /boot/efikeys
cd /boot/efikeys

openssl req -new -x509 -newkey rsa:2048 -subj "/CN=empoleos kernel-signing key/" -keyout kernel.key -out kernel.crt -days 3650 -nodes -sha256
#todo: consider the possible need to renew the openssl crt
# efikeygen --dbdir /boot/efikeys --self-sign --kernel --common-name 'CN=Empoleos signing key' --nickname 'Empoleos Secureboot'

chmod -v 400 *.key

# sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/BOOTX64.EFI" /boot/EFI/BOOT/BOOTX64.EFI
sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/grubx64.efi" /boot/EFI/BOOT/grubx64.efi

# refind-install ... --shim /usr/share/shim/BOOTX64.EFI ...
# refind-install --shim /usr/share/shim/BOOTX64.EFI
cp /usr/share/shim/BOOTX64.EFI /boot/EFI/BOOT/shimx64.efi
refind-install --shim /boot/EFI/BOOT/shimx64.efi

sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/grubx64.efi" /boot/EFI/BOOT/grubx64.efi

# sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/BOOTX64.EFI" /boot/EFI/BOOT/BOOTX64.EFI
# sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/mmx64.efi" /boot/EFI/BOOT/mmx64.efi
# sbsign --key /boot/efikeys/kernel.key --cert /boot/efikeys/kernel.crt --output="/boot/EFI/BOOT/shimx64.efi" /boot/EFI/BOOT/shimx64.efi


openssl x509 -inform pem -in /boot/efikeys/kernel.crt -outform der -out /boot/efikeys/sbcert.der

#todo: will ask to create password, which will need to be remembered on reboot
mokutil --import /boot/efikeys/sbcert.der


#!OLD - Below


# https://wiki.ubuntu.com/UEFI/SecureBoot/KeyManagement/KeyGeneration
mkdir -m 0700 /boot/efikeys
cd /boot/efikeys


# https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html/managing_monitoring_and_updating_the_kernel/signing-a-kernel-and-modules-for-secure-boot_managing-monitoring-and-updating-the-kernel
emerge app-crypt/pesign
#todo: may need to make a seperate secure boot method for arm
# ACCEPT_KEYWORDS="~amd64" make.conf

mkdir -p /etc/pki/pesign
chmod 700 /etc/pki/pesign

efikeygen --dbdir /etc/pki/pesign --self-sign --module --common-name 'CN=Empoleos signing key' --nickname 'Empoleos Secure Boot key'

#todo: enable secure boot ability
# https://wiki.gentoo.org/wiki/Secure_Boot#:~:text=Secure%20Boot%20is%20an%20enhancement,easily%20readable%2C%20but%20tamper%20evident.

emerge app-crypt/efitools
emerge app-crypt/sbsigntools
emerge sys-boot/mokutil
emerge dev-libs/openssl


# https://www.reddit.com/r/Gentoo/comments/uye0kv/secure_boot_with_sakakis_guide/
# https://www.youtube.com/watch?v=7SGM5cI7YhM
# https://wiki.gentoo.org/wiki/User:Sakaki/Sakaki%27s_EFI_Install_Guide/Configuring_Secure_Boot

# sbsign

# cat > /etc/dracut.conf.d/50-secure-boot.conf <<EOF uefi_secureboot_cert="/usr/share/secureboot/keys/db/db.pem" uefi_secureboot_key="/usr/share/secureboot/keys/db/db.key" EOF $ dracut -f --uefi --regenerate-all


mkdir -p -v /etc/efikeys
chmod 700 /etc/efikeys
cd /etc/efikeys

# backup old keys
efi-readvar -v PK -o old_PK.esl
efi-readvar -v KEK -o old_KEK.esl
efi-readvar -v db -o old_db.esl
efi-readvar -v dbx -o old_dbx.esl

# generate new keys
openssl req -new -x509 -newkey rsa:2048 -subj "/CN=$DistroNameLower platform key/" -keyout PK.key -out PK.crt -days 3650 -nodes -sha256
openssl req -new -x509 -newkey rsa:2048 -subj "/CN=$DistroNameLower key-exchange-key/" -keyout KEK.key -out KEK.crt -days 3650 -nodes -sha256
openssl req -new -x509 -newkey rsa:2048 -subj "/CN=$DistroNameLower kernel-signing key/" -keyout db.key -out db.crt -days 3650 -nodes -sha256
chmod -v 400 *.key

# preparing update files
cert-to-efi-sig-list -g "$(uuidgen)" db.crt db.esl
sign-efi-sig-list -a -k KEK.key -c KEK.crt db db.esl db.auth
cert-to-efi-sig-list -g "$(uuidgen)" PK.crt PK.esl
sign-efi-sig-list -k PK.key -c PK.crt PK PK.esl PK.auth
cert-to-efi-sig-list -g "$(uuidgen)" KEK.crt KEK.esl
sign-efi-sig-list -a -k PK.key -c PK.crt KEK KEK.esl KEK.auth

sign-efi-sig-list -k KEK.key -c KEK.crt dbx old_dbx.esl old_dbx.auth

# create DER versions
openssl x509 -outform DER -in PK.crt -out PK.cer
openssl x509 -outform DER -in KEK.crt -out KEK.cer
openssl x509 -outform DER -in db.crt -out db.cer

# compound old and new files
cat old_KEK.esl KEK.esl > compound_KEK.esl
cat old_db.esl db.esl > compound_db.esl
sign-efi-sig-list -k PK.key -c PK.crt KEK compound_KEK.esl compound_KEK.auth
sign-efi-sig-list -k KEK.key -c KEK.crt db compound_db.esl compound_db.auth

#note: will need to reboot system and enter setup mode
#google search: gentoo how to use efi-updatevar in user mode

# update key
efi-updatevar -e -f old_dbx.esl dbx
efi-updatevar -e -f compound_db.esl db
efi-updatevar -e -f compound_KEK.esl KEK
#todo: fix error "operation not permitted"
