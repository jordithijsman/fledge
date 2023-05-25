#!/bin/sh

# Set kubeconfig
export KUBECONFIG=$HOME/.kube/fledge2.yml
export IMAGE=papermc

# Deploy container
kubectl delete "configmaps/$IMAGE"
kubectl apply -f - << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: $IMAGE
data:
  banned-players.json: |
    []
  banned-ips.json: |
    []
  whitelist.json: |
    []
  server.properties: |
    #Minecraft server properties
    #(File Modification Datestamp)
    generator-settings=
    op-permission-level=4
    allow-nether=true
    level-name=world
    enable-query=false
    allow-flight=false
    announce-player-achievements=true
    server-port=25565
    max-world-size=29999984
    level-type=DEFAULT
    enable-rcon=false
    level-seed=
    force-gamemode=false
    server-ip=
    network-compression-threshold=256
    max-build-height=256
    spawn-npcs=true
    white-list=false
    spawn-animals=true
    hardcore=false
    snooper-enabled=true
    resource-pack-sha1=
    online-mode=true
    resource-pack=
    pvp=true
    difficulty=1
    enable-command-block=true
    gamemode=1
    player-idle-timeout=0
    max-players=20
    max-tick-time=60000
    spawn-monsters=true
    generate-structures=true
    view-distance=10
    motd=A Minecraft Server
EOF
