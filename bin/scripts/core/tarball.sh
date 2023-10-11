#!/bin/bash

dir="$1"
cpuType="$2"
cpuType2="$3"
stage3File="$4"
installID="$5"

cd /mnt/gentoo

if [ "$(ls "$dir/bin/cache/stage3-${cpuType2}-${stage3File}-${installID}" 2>/dev/null)" != "" ]; then
  cp "$dir/bin/cache/stage3-${cpuType2}-${stage3File}-${installID}" .
else
  # downlload and verify files
  wget "https://distfiles.gentoo.org/releases/${cpuType}/autobuilds/current-stage3-${cpuType2}-${stage3File}/stage3-${cpuType2}-${stage3File}-${installID}"
  wget "https://distfiles.gentoo.org/releases/${cpuType}/autobuilds/current-stage3-${cpuType2}-${stage3File}/stage3-${cpuType2}-${stage3File}-${installID}.sha256"

  if [ "$(sha256sum -c "stage3-${cpuType2}-${stage3File}-${installID}.sha256" 2>/dev/null)" = "" ]; then
    echo "Error: checksum failed!"

    rm -f "stage3-${cpuType2}-${stage3File}-${installID}"
    rm -f "stage3-${cpuType2}-${stage3File}-${installID}.sha256"

    exit
  fi

  wget "https://distfiles.gentoo.org/releases/${cpuType}/autobuilds/current-stage3-${cpuType2}-${stage3File}/stage3-${cpuType2}-${stage3File}-${installID}.CONTENTS.gz"
  wget "https://distfiles.gentoo.org/releases/${cpuType}/autobuilds/current-stage3-${cpuType2}-${stage3File}/stage3-${cpuType2}-${stage3File}-${installID}.DIGESTS"
  wget "https://distfiles.gentoo.org/releases/${cpuType}/autobuilds/current-stage3-${cpuType2}-${stage3File}/stage3-${cpuType2}-${stage3File}-${installID}.asc"

  if test -f "stage3-${cpuType2}-${stage3File}-${installID}.asc"; then
    wget -O - https://qa-reports.gentoo.org/output/service-keys.gpg | gpg --import &>/dev/null
    gpg --verify "stage3-${cpuType2}-${stage3File}-${installID}.asc" &>gpgtest.tmp
    if [ "$(cat gpgtest.tmp | grep "^gpg: Good [Ss]ignature")" = "" ]; then
      echo "Error: checksum failed!"

      rm -f gpgtest.tmp
      rm -f service-keys.gpg
      rm -f "stage3-${cpuType2}-${stage3File}-${installID}"
      rm -f "stage3-${cpuType2}-${stage3File}-${installID}.CONTENTS.gz"
      rm -f "stage3-${cpuType2}-${stage3File}-${installID}.sha256"
      rm -f "stage3-${cpuType2}-${stage3File}-${installID}.DIGESTS"
      rm -f "stage3-${cpuType2}-${stage3File}-${installID}.asc"

      exit
    fi

    rm -f gpgtest.tmp
    rm -f service-keys.gpg
  fi

  checkSums="$(sha512sum -c "stage3-${cpuType2}-${stage3File}-${installID}.DIGESTS" 2>/dev/null)"
  if [ "$(echo "$checkSums" | grep "^stage3-${cpuType2}-${stage3File}-${installID}: OK$")" = "" -o "$(echo "$checkSums" | grep "^stage3-${cpuType2}-${stage3File}-${installID}.CONTENTS.gz: OK$")" = "" ]; then
    echo "Error: checksum failed!"

    unset checkSums
    rm -f "stage3-${cpuType2}-${stage3File}-${installID}"
    rm -f "stage3-${cpuType2}-${stage3File}-${installID}.CONTENTS.gz"
    rm -f "stage3-${cpuType2}-${stage3File}-${installID}.sha256"
    rm -f "stage3-${cpuType2}-${stage3File}-${installID}.DIGESTS"
    rm -f "stage3-${cpuType2}-${stage3File}-${installID}.asc"

    exit
  fi

  unset checkSums
  rm -f "stage3-${cpuType2}-${stage3File}-${installID}.CONTENTS.gz"
  rm -f "stage3-${cpuType2}-${stage3File}-${installID}.sha256"
  rm -f "stage3-${cpuType2}-${stage3File}-${installID}.DIGESTS"
  rm -f "stage3-${cpuType2}-${stage3File}-${installID}.asc"

  rm -rf "$dir/bin/cache"
  mkdir -p "$dir/bin/cache"
  cp "stage3-${cpuType2}-${stage3File}-${installID}" "$dir/bin/cache/stage3-${cpuType2}-${stage3File}-${installID}"
fi

# unzip files
echo "Installing Tarball: $stage3File..."
tar xpf stage3-*.tar.xz --xattrs-include='*.*' --numeric-owner &>/dev/null
rm -f "stage3-${cpuType2}-${stage3File}-${installID}"
