#!/bin/sh

# Set kubeconfig
export KUBECONFIG=$HOME/.kube/fledge2.yml

# Deploy container
kubectl delete pods/papermc
kubectl apply -f - << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: papermc
spec:
  selector:
    matchLabels:
      run: papermc
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
  template:
    metadata:
      labels:
        run: papermc
    spec:
      containers:
      - name: papermc
        image: gitlab.ilabt.imec.be:4567/fledge/benchmark/papermc:1.10.2-capstan
        imagePullPolicy: Always
        ports:
        - containerPort: 25565
        workingDir: /data
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        hostPath:
          path: /srv/papermc/data
          type: DirectoryOrCreate
      nodeSelector:
        type: virtual-kubelet
      tolerations:
      - key: virtual-kubelet.io/provider
        operator: Equal
        value: backend
        effect: NoSchedule
---
apiVersion: v1
kind: Service
metadata:
  name: papermc
spec:
  ports:
  - port: 25565
    protocol: TCP
  selector:
    run: papermc
EOF
