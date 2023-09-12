#!/bin/bash

DistroName="$1"
DistroNameLower="$(echo "$DistroName" | tr '[:upper:]' '[:lower:]')"


#todo: enable secure boot ability
# https://wiki.gentoo.org/wiki/Secure_Boot#:~:text=Secure%20Boot%20is%20an%20enhancement,easily%20readable%2C%20but%20tamper%20evident.

emerge app-crypt/efitools
emerge app-crypt/sbsigntools
# emerge sys-boot/mokutil
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
cert-to-efi-sig-list -g "$(uuidgen)" PK.crt PK.esl
sign-efi-sig-list -k PK.key -c PK.crt PK PK.esl PK.auth
cert-to-efi-sig-list -g "$(uuidgen)" KEK.crt KEK.esl
sign-efi-sig-list -a -k PK.key -c PK.crt KEK KEK.esl KEK.auth
cert-to-efi-sig-list -g "$(uuidgen)" db.crt db.esl
sign-efi-sig-list -a -k KEK.key -c KEK.crt db db.esl db.auth

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
