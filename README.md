# Dovecot Director Controller

Dovecot director is used to keep a temporary user -> mail server (= dovecot server) mapping, as described in <a href="https://wiki.dovecot.org/Director">dovecot docs</a>.
If a dovecot director and dovecot server are used in a kubernetes cluster, mappings are not being updated in case a dovecot container restarts for example.
To update the new pod ip and therefore to correct the mapping a manual execution of the command "doveadm reload" needs to be done on the dovecot director server.
Since no one wants to waste manual effort on responses to ordinary container events this tool intends to automatically execute said command on the dovecot director shell whenever a dovecot container/pod becomes ready.


### Usage

Runs inside and outside of a kubernetes cluster.
If you don't run it inside a k8s cluster it tries to load the kubeconfig in the executing users homedir.
If it does not exist you need to specify the absolute path with command flag "-c".
 
Environment variables needed for successful execution:
* `DOVECOT_NAMESPACE`(string): Namespace name which must contain both dovecot director and dovecot pods
* `DOVECOT_LABELS`(string): All labels given to dovecot for conclusive identification of dovecot pods, same format as in `DOVECOT_DIRECTOR_LABELS`
* `DOVECOT_DIRECTOR_LABELS`(string): All labels given to dovecot director for conclusive identification of dovecot director pods in the following format: `<LABEL1>=<VALUE1>,<LABEL2>=<VALUE2>`
* `DOVECOT_DIRECTOR_CONTAINER_NAME` (string) (optional): Container Name of dovecot-director in Pod. Defaults to first Container in Pod if not set.

### Used Library
https://github.com/kubernetes/client-go
