#!/usr/bin/env python3
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

"""Back up, review, and migrate runtime overrides using existing PVC mounts.

Requires Python 3 and kubectl. Run without --apply first, review the private
backup directory (especially MS overrides), then rerun with --apply. Stop mutable
configuration updates and workload scaling/image changes during both steps.
No configuration values are printed or written to Kubernetes annotations.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import sys

POLICY = "doris.apache.org/runtime-config-policy"
MIGRATION = "doris.apache.org/runtime-config-migration"
STORE = ".doris-runtime-config"
KINDS = {"ddc": "dorisdisaggregatedclusters.disaggregated.cluster.doris.com",
         "dcr": "dorisclusters.doris.selectdb.com"}


def parse_conf(text):
    result = {}
    for line in text.splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            result[key.strip()] = value.strip()
    return result


def within(target, root):
    return target == root or target.startswith(root.rstrip("/") + "/")


def choose_mount(st, container, kind, config):
    """Keep ranking and exclusions in sync with runtimeconfig.selectStorage."""
    persistent = {v["name"] for v in st["spec"]["template"]["spec"].get("volumes", [])
                  if "persistentVolumeClaim" in v and not v["persistentVolumeClaim"].get("readOnly")}
    persistent.update(v["metadata"]["name"] for v in st["spec"].get("volumeClaimTemplates", []))
    roots = [entry["path"].replace("${DORIS_HOME}", "/opt/apache-doris/be").replace("$DORIS_HOME", "/opt/apache-doris/be") for entry in json.loads(config.get("file_cache_path", "[]"))]
    candidates = []
    for mount in container.get("volumeMounts", []):
        if mount["name"] not in persistent or mount.get("readOnly") or mount.get("subPathExpr"):
            continue
        path = mount["mountPath"].rstrip("/")
        rank = 0
        if PurePosixPath(path).name == "log" or path in (config.get("log_dir"), config.get("LOG_DIR")):
            rank = 1
        if kind == "fe" and (PurePosixPath(path).name == "doris-meta" or path == config.get("meta_dir")):
            rank = 2
        if container["name"].lower() == "be" and (PurePosixPath(path).name == "storage" or path in
                             [p.strip().split(",")[0] for p in config.get("storage_root_path", "").split(";")]):
            rank = 3
        if any(within(path, p) or within(p, path) for p in roots):
            rank = 0
        if rank:
            candidates.append((rank, path, mount))
    if not candidates:
        raise RuntimeError(f"{st['metadata']['name']}: no safe existing PVC; cache roots/emptyDir cannot be used")
    return min(candidates, key=lambda entry: entry[:2])[2]


def ms_overrides(effective, baseline):
    # FDB endpoint is injected by the entrypoint. Never freeze it in an override.
    ignored = {"fdb_cluster", "fdb_cluster_file_path", "custom_conf_path"}
    return "".join(f"{key} = {value}\n" for key, value in sorted(effective.items())
                   if key not in ignored and baseline.get(key) != value)


def private_write(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, "w") as stream:
        stream.write(data)


class Kubectl:
    def __init__(self, namespace, context):
        self.prefix = ["kubectl"]
        if context:
            self.prefix += ["--context", context]
        self.prefix += ["--namespace", namespace]

    def run(self, args, data=None):
        result = subprocess.run(self.prefix + args, input=data, text=True, capture_output=True)
        if result.returncode:
            # Do not propagate stdout/stderr: API/exec failures can contain config values.
            raise RuntimeError(f"kubectl {args[0]} failed (exit {result.returncode}); inspect the cluster separately")
        return result.stdout

    def get(self, kind, name=None):
        return json.loads(self.run(["get", kind] + ([name] if name else []) + ["-o", "json"]))

    def read(self, pod, container, path):
        return self.run(["exec", pod, "-c", container, "--", "bash", "-ec",
                         'if [[ -f "$1" ]]; then cat -- "$1"; fi', "read-config", path])


def controlled(resource, uid):
    return any(ref["uid"] == uid
               for ref in resource["metadata"].get("ownerReferences", []))


def collect(kube, kind, name):
    cluster = kube.get(KINDS[kind], name)
    if cluster["metadata"].get("annotations", {}).get(POLICY) in ("enabled", "auto"):
        raise RuntimeError("cluster has already enabled automatic persistence; do not migrate active overrides")
    workloads = [s for s in kube.get("statefulsets")["items"] if controlled(s, cluster["metadata"]["uid"])]
    if not workloads:
        raise RuntimeError("no existing StatefulSets found")
    pods = kube.get("pods")["items"]
    plan = {"clusterUID": cluster["metadata"]["uid"], "workloads": {}, "podUIDs": {}, "pods": []}
    for st in workloads:
        containers = st["spec"]["template"]["spec"]["containers"]
        main = next((c for c in containers if c["name"].lower() in ("fe", "be", "compute", "metaservice")), None)
        if not main:
            continue
        comp = {"compute": "be", "metaservice": "ms"}.get(main["name"].lower(), main["name"].lower())
        meta = st["metadata"]
        replicas = st["spec"].get("replicas", 1)
        status = st.get("status", {})
        if replicas and (status.get("observedGeneration", 0) < meta["generation"] or
                         status.get("currentRevision") != status.get("updateRevision")):
            raise RuntimeError(f"{meta['name']}: wait for the existing rollout to finish")
        members = [p for p in pods if controlled(p, meta["uid"]) and not p["metadata"].get("deletionTimestamp")]
        if len(members) != replicas:
            raise RuntimeError(f"{meta['name']}: replicas are not stable")
        plan["workloads"][meta["name"]] = {"uid": meta["uid"], "generation": meta["generation"], "replicas": replicas}
        for pod in members:
            if status.get("updateRevision") and pod["metadata"].get("labels", {}).get("controller-revision-hash") != status["updateRevision"]:
                raise RuntimeError(f"{pod['metadata']['name']}: Pod is not at the current StatefulSet revision")
            pod_name = pod["metadata"]["name"]
            plan["podUIDs"][pod_name] = pod["metadata"]["uid"]
            if not any(c.get("type") == "Ready" and c.get("status") == "True" for c in pod.get("status", {}).get("conditions", [])):
                raise RuntimeError(f"{pod_name}: Pod must be Ready before migration")
            home = "/opt/apache-doris/" + comp
            base = "doris_cloud.conf" if comp == "ms" else comp + ".conf"
            base_text = kube.read(pod_name, main["name"], home + "/conf/" + base)
            if not base_text:
                raise RuntimeError(f"{pod_name}: base configuration is missing")
            config = parse_conf(base_text)
            # An existing explicit path needs individual verification, not relocation.
            key = "custom_conf_path" if comp == "ms" else "custom_config_dir"
            if config.get(key) and config[key] not in (home + "/conf", "${DORIS_HOME}/conf", home + "/conf/" + base):
                raise RuntimeError(f"{pod_name}: custom configuration path already set; verify/migrate it separately")
            mount = choose_mount(st, main, comp, config)
            filename = comp + "_custom.conf"
            if comp == "ms":
                # Start with the original ConfigMap, or the entrypoint's image backup.
                source = next((e["value"] for e in main.get("env", []) if e["name"] == "CONFIGMAP_PATH" and e.get("value")), "/etc/doris")
                baseline = kube.read(pod_name, main["name"], source + "/" + base)
                if not baseline:
                    raise RuntimeError(f"{pod_name}: MS baseline ConfigMap unavailable; export reviewed MS overrides manually before migration")
                contents = ms_overrides(config, parse_conf(baseline))
                source_path = home + "/conf/" + base
                source_text = base_text
            else:
                source_path = home + "/conf/" + filename
                source_text = kube.read(pod_name, main["name"], source_path)
                contents = source_text
                if comp == "be" and config.get("deploy_mode") == "cloud":
                    persisted = dict(config)
                    persisted.update(parse_conf(contents))
                    if not persisted.get("meta_service_endpoint") or not persisted.get("cloud_unique_id"):
                        raise RuntimeError(f"{pod_name}: persisted cloud heartbeat configuration is incomplete; recover it before migration")
            target = mount["mountPath"].rstrip("/") + "/" + STORE + "/" + pod_name
            plan["pods"].append({"pod": pod_name, "uid": pod["metadata"]["uid"], "container": main["name"],
                                 "file": pod_name + ".conf", "contents": contents,
                                 "source": source_path, "sourceSHA256": hashlib.sha256(source_text.encode()).hexdigest(),
                                 "target": target, "filename": filename})
    return plan


def apply(kube, kind, name, directory, saved):
    # Recollect before touching a PVC. This rejects replaced Pods, scale/image
    # changes, and dynamic updates that occurred since the backup was reviewed.
    live = collect(kube, kind, name)
    keys = ("clusterUID", "workloads", "podUIDs")
    if any(saved[k] != live[k] for k in keys):
        raise RuntimeError("cluster/workload changed since backup; make a fresh backup")
    current = {p["pod"]: p for p in live["pods"]}
    for pod in saved["pods"]:
        now = current.get(pod["pod"])
        if not now or any(pod[k] != now[k] for k in ("uid", "sourceSHA256", "target", "filename")):
            raise RuntimeError(f"{pod['pod']}: Pod or runtime configuration changed; make a fresh backup")
    if len(current) != len(saved["pods"]):
        raise RuntimeError("Pod membership changed since backup")
    for pod in saved["pods"]:
        contents = (directory / pod["file"]).read_text()
        # No shell expansion of file paths or contents.
        kube.run(["exec", "-i", pod["pod"], "-c", pod["container"], "--", "bash", "-ec", '''
set -euo pipefail
umask 077
mkdir -p -- "$1"
tmp=$(mktemp "$1/.migration.XXXXXX")
trap 'rm -f -- "$tmp"' EXIT
cat > "$tmp"
if [[ -e "$1/$2" ]]; then
    cmp -s -- "$tmp" "$1/$2" || exit 1
else
    mv -- "$tmp" "$1/$2"
fi
: > "$1/.migration-complete"
''', "migrate-config", pod["target"], pod["filename"]], data=contents)
        # Read back without printing the content.
        actual = kube.read(pod["pod"], pod["container"], pod["target"] + "/" + pod["filename"])
        if actual != contents:
            raise RuntimeError(f"{pod['pod']}: backup verification failed")
    # Make sure no image/replica/Pod change happened while copying files.
    final = collect(kube, kind, name)
    if saved["workloads"] != final["workloads"] or {(p["pod"], p["uid"], p["sourceSHA256"]) for p in saved["pods"]} != {(p["pod"], p["uid"], p["sourceSHA256"]) for p in final["pods"]}:
        raise RuntimeError("cluster changed during migration; persistence was not enabled")
    approval = {k: saved[k] for k in keys}
    patch = {"metadata": {"annotations": {MIGRATION: json.dumps(approval, separators=(",", ":"))}},
             "spec": {"runtimeConfigPersistence": {"enabled": True}}}
    kube.run(["patch", KINDS[kind], name, "--type=merge", "-p", json.dumps(patch)])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kind", required=True, choices=KINDS)
    parser.add_argument("--name", required=True)
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--context")
    parser.add_argument("--backup-dir", required=True, type=Path)
    parser.add_argument("--apply", action="store_true", help="apply a previously reviewed backup and enable persistence")
    args = parser.parse_args()
    kube = Kubectl(args.namespace, args.context)
    manifest = args.backup_dir / "manifest.json"
    if args.apply:
        saved = json.loads(manifest.read_text())
        if saved.get("kind") != args.kind or saved.get("name") != args.name or saved.get("namespace") != args.namespace:
            raise RuntimeError("backup belongs to another cluster")
        apply(kube, args.kind, args.name, args.backup_dir, saved)
        print("Migration verified; persistence enabled. Watch the normal rolling update.")
    else:
        if args.backup_dir.exists():
            raise RuntimeError("use a new backup directory; existing backups are never overwritten")
        saved = collect(kube, args.kind, args.name)
        args.backup_dir.mkdir(mode=0o700, parents=True)
        for pod in saved["pods"]:
            private_write(args.backup_dir / pod["file"], pod.pop("contents"))
        saved.update(kind=args.kind, name=args.name, namespace=args.namespace)
        private_write(manifest, json.dumps(saved, indent=2))
        print(f"Backed up {len(saved['pods'])} Pods. Review the private files, then rerun with --apply.")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, KeyError) as error:
        print(f"Migration stopped: {error}", file=sys.stderr)
        sys.exit(1)
