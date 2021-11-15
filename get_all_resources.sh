#!/bin/bash
namespaces=("default" "ttt")

for namespace in ${namespaces[@]};do
    echo "$namespace"
    kubectl get deployment -n "$namespace"
    kubectl get IdleOperator -n "$namespace"
done
