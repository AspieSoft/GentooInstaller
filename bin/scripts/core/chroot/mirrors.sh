#!/bin/bash

continent="$1"

# get closest mirrors in region
echo "finding fastest mirrors in your region..."
mirrorSize="9"
mirrors="$(mirrorselect -s$mirrorSize -R "$continent" -o 2>/dev/null)"
while [ "$mirrors" = "" ]; do
  mirrorSize="$((mirrorSize - 1))"
  if [ "$mirrorSize" -lt "1" ]; then
    echo "Error: Failed To Find Any Gentoo Mirrors In Your Region!"
    break
  fi

  mirrors="$(mirrorselect -s$mirrorSize -R "$continent" -o 2>/dev/null)"
done

# get closest mirrors
if [ "$mirrors" = "" ]; then
  echo "Trying Mirrors Outside Your Region..."

  mirrorSize="9"
  mirrors="$(mirrorselect -s$mirrorSize -o 2>/dev/null)"
  while [ "$mirrors" = "" ]; do
    mirrorSize="$((mirrorSize - 1))"
    if [ "$mirrorSize" -lt "1" ]; then
      echo "Error: Failed To Find Any Gentoo Mirrors!"
      exit
    fi

    mirrors="$(mirrorselect -s$mirrorSize -o 2>/dev/null)"
  done
fi

echo -e "$mirrors" >> etc/portage/make.conf
unset mirrors

# copy repos.conf from /usr/share/portage to /etc/portage
mkdir -p /etc/portage/repos.conf
cp /usr/share/portage/config/repos.conf /etc/portage/repos.conf/gentoo.conf
