# Testing the iscsi provisioner

## What is it ?

This is a very simple example on how to test the iscsi CSI which  
basically creates a presistent volume claim pointing to the storageclass  
and a pod that runs and utilized the PVC

## how to Run :

```
oc apply -f iscsi-pvc.yaml -f iscsi-test-pod.yaml
```

No run the pods list to show everything is running :

```
oc get pods
```

That's it !!!
