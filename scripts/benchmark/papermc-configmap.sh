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
  ops.json: |
    [
        {
            "uuid": "ec9e6643-8bdb-3fc2-977d-e73f0bb1cb29",
            "name": "MaxDuClark",
            "level": 4
        },
        {
            "uuid": "56b13711-e1be-3ae5-954f-390c9ebab470",
            "name": "Player 0",
            "level": 4
        },
        {
            "uuid": "7d47ab93-4c5b-30ee-abb9-99f58321567e",
            "name": "Player 1",
            "level": 4
        },
        {
            "uuid": "b58e6638-c98b-3fe8-b302-13fb46110906",
            "name": "Player 2",
            "level": 4
        },
        {
            "uuid": "398be230-de2c-3009-aa1a-ecd53565ed8a",
            "name": "Player 3",
            "level": 4
        },
    ]
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
    level-seed=42
    force-gamemode=false
    server-ip=
    network-compression-threshold=256
    max-build-height=256
    spawn-npcs=false
    white-list=false
    spawn-animals=false
    hardcore=false
    snooper-enabled=true
    resource-pack-sha1=
    online-mode=false
    resource-pack=
    pvp=true
    difficulty=1
    enable-command-block=true
    gamemode=1
    player-idle-timeout=0
    max-players=20
    max-tick-time=60000
    spawn-monsters=false
    generate-structures=true
    use-native-transport=false
    view-distance=10
    motd=A Minecraft Server
EOF
