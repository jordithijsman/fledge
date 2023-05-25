#!/bin/sh

# Set kubeconfig
export KUBECONFIG=$HOME/.kube/fledge2.yml
export IMAGE=papermc
export VERSION=1.10.2-prom
export BACKEND=capstan

# Deploy container
kubectl delete "deployments/$IMAGE-$BACKEND"
kubectl apply -f - << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $IMAGE-$BACKEND
spec:
  selector:
    matchLabels:
      run: $IMAGE-$BACKEND
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
  template:
    metadata:
      labels:
        run: $IMAGE-$BACKEND
    spec:
      containers:
      - name: app
        image: gitlab.ilabt.imec.be:4567/fledge/benchmark/$IMAGE:$VERSION-$BACKEND
        imagePullPolicy: Always
        ports:
        - containerPort: 9940
          hostPort: 9940
        - containerPort: 25565
          hostPort: 25565
        workingDir: /data
        volumeMounts:
        - name: config
          mountPath: /
        - name: data
          mountPath: /data
      volumes:
      - name: config
        configMap:
          name: papermc
      - name: data
        hostPath:
          path: /srv/$IMAGE-$BACKEND/data
          type: DirectoryOrCreate
      nodeSelector:
        type: virtual-kubelet
      tolerations:
      - key: virtual-kubelet.io/provider
        operator: Equal
        value: broker
        effect: NoSchedule
EOF
