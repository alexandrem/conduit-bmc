# OpenBMC

## Building QEMU ARM Image Artifacts

> NOTE: Linux only

Deps:

```bash
sudo apt update && sudo apt install -y gawk wget git-core diffstat unzip
texinfo      gcc build-essential chrpath socat cpio python3 python3-pip python3-pexpect      xz-utils debianutils iputils-ping python3-git python3-jinja2      libegl1-mesa libsdl1.2-dev xterm locales

sudo apt-get update && sudo apt-get install -y \
    file \
    zstd \
    lz4 \
    zstd

sudo apt-get install -y \
    build-essential \
    bzip2 \
    gzip \
    xz-utils \
    cpio \
    chrpath \
    diffstat \
    texinfo \
    file \
    python3 \
    python3-pip \
    python3-venv \
    unzip \
    curl \
    git

sudo locale-gen en_US.UTF-8

# for ubuntu 24.04
echo "kernel.apparmor_restrict_unprivileged_userns=0" | sudo tee -a /etc/sysctl.conf

sudo useradd -ms /bin/bash yocto
sudo su - yocto
```

Then using `yocto` user:

```bash
git clone https://github.com/openbmc/openbmc
cd openbmc
source setup qemuarm
bitbake obmc-phosphor-image
```
