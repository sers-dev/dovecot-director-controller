# Dovecot Director Controller

Dovecot director is used to keep a temporary user -> mail server (= dovecot server) mapping, as described in
[dovecot docs](https://wiki.dovecot.org/Director).

If a dovecot director and dovecot server are used in a kubernetes cluster, mappings are not being updated in case a 
dovecot container restarts for example since dovecot is not made to be used with private IPs as it is in k8s setup.
To update the new pod ip and therefore to correct the mapping, a manual execution of the command `doveadm reload` needs 
to be done on the dovecot director server.

Since no one wants to waste manual effort on responses to ordinary container events this tool intends to automatically
execute said command on the dovecot director pod container shell whenever a dovecot container/pod becomes ready 
or when the tls secret is changed or updated.


### Usage

Runs inside and outside of a kubernetes cluster.

If you don't run it inside a k8s cluster it tries to load the kubeconfig in the executing users homedir.
If it does not exist you need to specify the absolute path with command flag "-c".
 
Environment variables needed for successful execution:
* `DOVECOT_NAMESPACE`(string): Name of namespace that must contain both dovecot and dovecot director pods
* `DOVECOT_DIRECTOR_LABELS`(string): All labels given to dovecot director for conclusive identification of dovecot director pods in the following format: `<LABEL1>=<VALUE1>,<LABEL2>=<VALUE2>`
* `DOVECOT_LABELS`(string): All labels given to dovecot for conclusive identification of dovecot pods, same format as in `DOVECOT_DIRECTOR_LABELS`

  Optional environment parameters:
* `DOVECOT_DIRECTOR_CONTAINER_NAME` (string) : Container Name of dovecot-director in Pod. Defaults to first Container in Pod if not set.
* `SYNC_FREQUENCY_DURATION` (int, seconds, default: 70): This parameter is based on kubelets parameter `--sync-frequency duration` which is 
    by default set to 60s, so if you change the value of kubelets parameter you should use the same value plus a few seconds
    to trigger at `doveadm reload` after adding/changing tls secrets successfully

When running inside a kubernetes cluster the pod needs the following permissions via Role for the same namespace your 
dovecot pods run in:
```
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - secrets
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - pods/exec
  verbs:
  - create
```

### Used Library
https://github.com/kubernetes/client-go
