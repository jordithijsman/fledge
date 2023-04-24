#!/bin/sh
set -xe
port=$(ssh worker0 ps aux | grep qemu-system-x86_64 | tail -1 | grep -oP 'hostfwd=tcp::\K[0-9]+')
ssh -L 127.0.0.1:25565:worker0.fledge2.ilabt-imec-be.wall1.ilabt.iminds.be:$port worker0 sleep 1000000
