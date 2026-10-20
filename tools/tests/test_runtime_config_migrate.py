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

import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("migrate", Path(__file__).parents[1] / "runtime-config-migrate.py")
migrate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(migrate)


class FakeKubectl:
    def __init__(self):
        self.cluster = {"metadata": {"uid": "cluster-uid"}}
        self.st = {"metadata": {"name": "cluster-be", "uid": "st-uid", "generation": 2,
                               "ownerReferences": [{"uid": "cluster-uid"}]},
                   "status": {"observedGeneration": 2, "currentRevision": "rev", "updateRevision": "rev"},
                   "spec": {"replicas": 1, "template": {"spec": {
                       "containers": [{"name": "BE", "volumeMounts": [{"name": "storage", "mountPath": "/storage"}]}],
                       "volumes": [{"name": "storage", "persistentVolumeClaim": {"claimName": "disk"}}]}}}}
        self.pod = {"metadata": {"name": "cluster-be-0", "uid": "pod-uid", "labels": {"controller-revision-hash": "rev"},
                                "ownerReferences": [{"controller": True, "uid": "st-uid"}]},
                    "status": {"conditions": [{"type": "Ready", "status": "True"}]}}
        self.files = {"/opt/apache-doris/be/conf/be.conf": "storage_root_path=/storage\n",
                      "/opt/apache-doris/be/conf/be_custom.conf": "mutable=value\n"}
        self.patched = False

    def get(self, kind, name=None):
        if kind in migrate.KINDS.values():
            return self.cluster
        if kind == "statefulsets":
            return {"items": [self.st]}
        if kind == "pods":
            return {"items": [self.pod]}
        raise AssertionError(kind)

    def read(self, pod, container, path):
        return self.files.get(path, "")

    def run(self, args, data=None):
        if args[0] == "exec":
            self.files[args[-2] + "/" + args[-1]] = data
        elif args[0] == "patch":
            self.patched = json.loads(args[-1])
        return ""


class MigrationTest(unittest.TestCase):
    def test_ms_excludes_base_and_fdb(self):
        result = migrate.ms_overrides({"port": "5000", "mutable": "new", "fdb_cluster": "injected"},
                                      {"port": "5000", "mutable": "old"})
        self.assertEqual(result, "mutable = new\n")

    def test_cache_only_is_rejected(self):
        kube = FakeKubectl()
        with self.assertRaisesRegex(RuntimeError, "no safe"):
            migrate.choose_mount(kube.st, kube.st["spec"]["template"]["spec"]["containers"][0],
                                 "be", {"file_cache_path": '[{"path":"/storage"}]'})

    def test_collect_and_apply(self):
        kube = FakeKubectl()
        plan = migrate.collect(kube, "dcr", "cluster")
        self.assertEqual(plan["podUIDs"], {"cluster-be-0": "pod-uid"})
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for pod in plan["pods"]:
                migrate.private_write(root / pod["file"], pod.pop("contents"))
                self.assertEqual((root / pod["file"]).stat().st_mode & 0o777, 0o600)
            migrate.apply(kube, "dcr", "cluster", root, plan)
        self.assertTrue(kube.patched["spec"]["runtimeConfigPersistence"]["enabled"])
        self.assertNotIn("mutable", kube.patched["metadata"]["annotations"][migrate.MIGRATION])

    def test_replaced_pod_never_enables(self):
        kube = FakeKubectl()
        plan = copy.deepcopy(migrate.collect(kube, "dcr", "cluster"))
        kube.pod["metadata"]["uid"] = "replacement"
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(RuntimeError, "changed"):
                migrate.apply(kube, "dcr", "cluster", Path(directory), plan)
        self.assertFalse(kube.patched)

    def test_dynamic_change_never_enables(self):
        kube = FakeKubectl()
        plan = migrate.collect(kube, "dcr", "cluster")
        kube.files["/opt/apache-doris/be/conf/be_custom.conf"] = "changed=yes\n"
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(RuntimeError, "changed"):
                migrate.apply(kube, "dcr", "cluster", Path(directory), plan)
        self.assertFalse(kube.patched)


if __name__ == "__main__":
    unittest.main()
