#!/bin/bash

if type update-alternatives 2>/dev/null >&1; then
    # Remove previous link if it doesn't use update-alternatives
    if [ -L '/usr/bin/force-terminal' -a -e '/usr/bin/force-terminal' -a "`readlink '/usr/bin/force-terminal'`" != '/etc/alternatives/force-terminal' ]; then
        rm -f '/usr/bin/force-terminal'
    fi
    update-alternatives --install '/usr/bin/force-terminal' 'force-terminal' '/opt/Force Terminal/force-terminal' 100 || ln -sf '/opt/Force Terminal/force-terminal' '/usr/bin/force-terminal'
else
    ln -sf '/opt/Force Terminal/force-terminal' '/usr/bin/force-terminal'
fi

chmod 4755 '/opt/Force Terminal/chrome-sandbox' || true

if hash update-mime-database 2>/dev/null; then
    update-mime-database /usr/share/mime || true
fi

if hash update-desktop-database 2>/dev/null; then
    update-desktop-database /usr/share/applications || true
fi
