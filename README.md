# kube-applier

[![Docker Repository on Quay](https://quay.io/repository/utilitywarehouse/kube-applier/status "Docker Repository on Quay")](https://quay.io/repository/utilitywarehouse/kube-applier)

Table of Contents
=================

   * [kube-applier](#kube-applier)
   * [Table of Contents](#table-of-contents)
      * [Usage](#usage)
         * [Environment variables](#environment-variables)
         * [Annotations](#annotations)
         * [Mounting the Git Repository](#mounting-the-git-repository)
      * [Deploying](#deploying)
      * [Monitoring](#monitoring)
         * [Status UI](#status-ui)
         * [Metrics](#metrics)
      * [Running locally](#running-locally)
      * [Copyright and License](#copyright-and-license)

Created by [gh-md-toc](https://github.com/ekalinin/github-markdown-toc)

Forked from: https://github.com/box/kube-applier

kube-applier is Kubernetes deployment tool strongly following
[gitOps](https://www.weave.works/blog/gitops-operations-by-pull-request)
principals. It enables continuous deployment of Kubernetes objects by applying
declarative configuration files from a Git repository to a Kubernetes cluster.

kube-applier runs as a Deployment in your cluster and watches the [Git
repo](#mounting-the-git-repository) to ensure that the cluster objects are
up-to-date with their associated spec files (JSON or YAML) in the repo.

Whenever a new commit to the repo occurs, or at a [specified
interval](#run-interval), kube-applier performs a "run", issuing [kubectl
apply](https://kubernetes.io/docs/user-guide/kubectl/v1.6/#apply) commands at
namespace level. The convention is that level 1 subdirs of REPO_PATH represent
k8s namespaces: the name of the dir is the same as the namespace and the dir
contains manifests for the given namespace.

kube-applier serves a [status page](#status-ui) and provides
[metrics](#metrics) for monitoring.

## Usage

### Environment variables

**Required:**
* `REPO_PATH` - (string) Absolute path to the directory containing
  configuration files to be applied. It must be a Git repository or a path within
  one. Level 1 subdirs of this directory represent kubernetes namespaces.

**Optional:**

* `DIFF_URL_FORMAT` should be a URL for a hosted remote repo that supports
  linking to a commit hash. Replace the commit hash portion with "%s" so it can
  be filled in by kube-applier (e.g.
  `https://github.com/kubernetes/kubernetes/commit/%s`).

* `LISTEN_PORT` - (int) Port for the container. This should be the same port
  specified in the container spec. Default is 8080.

* `REPO_PATH_FILTERS` - (string) A comma separated list of sub directories to
  be applied. Supports [shell file name
  patterns](https://golang.org/pkg/path/filepath/#Match).

* `SERVER` - (string) Address of the Kubernetes API server. By default,
  kube-applier uses in-cluster configuration targetting local internal endpoint.
  Address must be specified with this environment variable (which is then written
  into a [kubeconfig
  file](http://kubernetes.io/docs/user-guide/kubeconfig-file/)). Authentication
  to the API server is handled by service account tokens. See [Accessing the
  Cluster](http://kubernetes.io/docs/user-guide/accessing-the-cluster/#accessing-the-api-from-a-pod)
  for more info.

* `POLL_INTERVAL_SECONDS` - (int) Number of seconds to wait between each check
  for new commits to the repo (default is 5).

* <a name="run-interval"></a>`FULL_RUN_INTERVAL_SECONDS` - (int) Number of
  seconds between automatic full runs (default is 3600). Set to 0 to disable.

* `DRY_RUN` - (bool) If true, kubectl command will be run with --server-dry-run
  flag. This means live configuration of the cluster is not changed.

* `LOG_LEVEL` - (string) trace|debug|info|warn|error case insensitive

### Annotations

kube-applier behaviour is controlled through annotations on the Namespace
resource.

```
kind: Namespace
apiVersion: v1
metadata:
  name: team
  annotations:
    kube-applier.io/enabled: 'true'
    kube-applier.io/dry-run: 'false'
    kube-applier.io/prune: 'true'
```

### Mounting the Git Repository

Git-sync keeps a local directory up to date with a remote repo. The local
directory resides in a shared emptyDir volume that is mounted in both the
git-sync and kube-applier containers.

Reference the [git-sync](https://github.com/kubernetes/git-sync) repo for setup
and usage.

**What happens if the contents of the local Git repo change in the middle of a kube-applier run?**

If there are changes to files in the `$REPO_PATH` directory during a
kube-applier run, those changes may or may not be reflected in that run,
depending on the timing of the changes.

Given that the `$REPO_PATH` directory is a Git repo or located within one, it
is likely that the majority of changes will be associated with a Git commit.
Thus, a change in the middle of a run will likely update the HEAD commit hash,
which will immediately trigger another run upon completion of the current run
(regardless of whether or not any of the changes were effective in the current
run). However, changes that are not associated with a new Git commit will not
trigger a run.

**If I remove a configuration file, will kube-applier remove the associated Kubernetes object?**

This is dependent on the `kube-applier.io/prune` value (default true). If true,
then the
[pruneWhitelist](https://github.com/utilitywarehouse/kube-applier/blob/master/kube/client.go#L37-L49)
is passed to the `apply` command.

## Deploying

Included is a Kustomize (https://kustomize.io/) base you can reference in your
namespace:

```
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
- github.com/utilitywarehouse/kube-applier//manifests/base?ref=2.3.6
```

and patch as per example:
[manifests/example/](manifests/example/)

Please note that if you enable kustomize for your namespace and you've enabled pruning in kube-applier, _all_ your resources need to be listed in your `kustomization.yaml` under `resources`. If you don't do this kube-applier will assume they have been removed and start pruning. 

## Monitoring
### Status UI
![screenshot](https://github.com/box/kube-applier/raw/master/static/img/status_page_screenshot.png "Status Page Screenshot")

kube-applier hosts a status page on a webserver, served at the service endpoint
URL. The status page displays information about the most recent apply run,
including:
* Start and end times
* Latency
* Most recent commit
* Blacklisted files
* Errors
* Files applied successfully

The HTML template for the status page lives in `templates/status.html`, and `static/` holds additional assets.

### Metrics
kube-applier uses [Prometheus](https://github.com/prometheus/client_golang) for
metrics. Metrics are hosted on the webserver at /metrics (status UI is the
index page). In addition to the Prometheus default metrics, the following
custom metrics are included:

* **run_latency_seconds** - A
  [Summary](https://godoc.org/github.com/prometheus/client_golang/prometheus#Summary)
  that keeps track of the durations of each apply run, tagged with a boolean for
  whether or not the run was a success (i.e. no failed apply attempts).

* **namespace_apply_count** - A
  [Counter](https://godoc.org/github.com/prometheus/client_golang/prometheus#Counter)
  for each namespace that has had an apply attempt over the lifetime of the
  container, incremented with each apply attempt and tagged by the namespace and
  the result of the attempt.

* **result_summary** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  for each deployment, labelled with the namespace, action, status and type of
  object applied

* **kubectl_exit_code_count** - A
  [Counter](https://godoc.org/github.com/prometheus/client_golang/prometheus#Counter)
  for each exit code returned by executions of `kubectl`, labelled with the
  namespace and exit code.

The Prometheus [HTTP API](https://prometheus.io/docs/querying/api/) (also see
the [Go
library](https://github.com/prometheus/client_golang/tree/master/api/prometheus))
can be used for querying the metrics server.

## Running locally

```
# manifests git repository
export LOCAL_REPO_PATH="${HOME}/dev/work/kubernetes-manifests"

# directory within the manifests repository that contains namespace directories
export CLUSTER_DIR="exp-1-aws"

export DIFF_URL_FORMAT="https://github.com/utilitywarehouse/kubernetes-manifests/commit/%s"
export REPO_PATH_FILTERS="sys-*,kube-system,labs"
export LOG_LEVEL="info"
export DRY_RUN="true"
export SERVER="https://api.server"
```

The ca.crt and token to use against the remote server. This can be the
ServiceAccount that KA runs with, or any SA that contains enough privileges

```
sa=$(kubectl --context=exp-1-aws -n sys-kube-applier get secret | grep kube-applier | awk '{print $1}')
kubectl --context=exp-1-aws -n sys-kube-applier get secret ${sa} -o json | jq -r '."data"."ca.crt"' | base64 -d > /tmp/ka-ca.crt
kubectl --context=exp-1-aws -n sys-kube-applier get secret ${sa} -o json | jq -r '."data"."token"' | base64 -d > /tmp/ka-token
```

```
make build
make run
```

## Copyright and License

Copyright 2016 Box, Inc. All rights reserved.

Copyright (c) 2017-2019 Utility Warehouse Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
