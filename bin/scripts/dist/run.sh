#!/bin/bash

dir="$1"

# set bash PS1
echo >> 'if [ "$PS1" ]; then' /etc/profile.d/bash_ps.sh
echo >> '  PS1="\[\e[m\][\[\e[1;32m\]\u@\h\[\e[m\]:\[\e[1;34m\]\w\[\e[m\]]\[\e[0;31m\](\$?)\[\e[1;0m\]\\$ \[\e[m\]"' /etc/profile.d/bash_ps.sh
echo >> 'fi' /etc/profile.d/bash_ps.sh

#todo: install desktop|server environment
#*note: files within the desktop|server directories will get merged into the root dist directory when copied over
#* only the file for the apropriate type will be merged, and will overwrite any same named files already in the dist directory
