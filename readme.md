# Gentoo Installer

[![donation link](https://img.shields.io/badge/buy%20me%20a%20coffee-paypal-blue)](https://paypal.me/shaynejrtaylor?country.x=US&locale.x=en_US)

An automation of the gentoo installation process for linux.

## Notice: This Project Is Still In Beta!

## Installation

```shell script
git clone https://github.com/AspieSoft/GentooInstaller.git

# run installation wizard
sudo GentooInstaller/install.sh

# server install
sudo GentooInstaller/install.sh --server

# use default locale
sudo GentooInstaller/install.sh -L

# set locale
sudo GentooInstaller/install.sh --locale=en_US
```

## Config

In the `config` directory, you can edit `DistroName.conf` to change the name of your custom gentoo based distro.

You can also edit `tarball.conf` to change the order of the chosen stage3 tarball. If a file does not exist, it moves on to the next one.

Note: If server mode fails to find a tarball file on gentoo servers, it will fallback to the client tarball files. The client will Not fallback to the server however, because some server tarballs are to strict for a GUI.

You can also configure the `bin/scripts/dist` files, and the `bin/assets` directory.
The file `bin/scripts/dist/run.sh` will be called after the distro is done installing, and within chroot, so you can run your custom setup scripts.

The root for these scripts will be `/gentoo-installer/scripts/dist`, and your assets will be in the `/gentoo-installer/assets` directory.

Note: the `/gentoo-installer/assets/theme` directory will already have been removed. The files `backgrounds.tar.gz`, `icons.tar.gz`, `sounds.tar.gz`, and `themes.tar.gz` will have been already been unzipped into the `/usr/share` directory.

There is one more file, which I do Not recommend editing too much. The file `bin/scripts/core/disk.yml` is currently very limited in functionality, and editing this file may also require some changes to the `bin/scripts/core/disk.sh` file for things to work as expected.
