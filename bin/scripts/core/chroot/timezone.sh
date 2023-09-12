#!/bin/bash

timezone="$1"
locale="$2"


echo "$timezone" > /etc/timezone

emerge --config sys-libs/timezone-data

echo "$locale ISO-8859-1" | tee -a /etc/locale.gen
echo "$locale.UTF-8 UTF-8" | tee -a /etc/locale.gen

locale-gen

eselect locale set "$locale.utf8"
