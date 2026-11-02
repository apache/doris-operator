# FoundationDB Operator v2.35.0 installation assets

Source: https://github.com/FoundationDB/fdb-kubernetes-operator/tree/fd8fdbd2c6d14375116e97977f1f49e3d17b84a3

CRDs are copied byte-for-byte from this revision (Apache-2.0). SHA-256:

- `apps.foundationdb.org_foundationdbclusters.yaml`: `e6921ca3fb10559adb71e8e1836b284df92627d54147f011b1f23863ee636909`
- `apps.foundationdb.org_foundationdbbackups.yaml`: `0a4e293c97d8e400cf833d76869235c48e421e8a16ec657ccb75f60ad1b00898`
- `apps.foundationdb.org_foundationdbrestores.yaml`: `6195625023dadcc5804c0556c2dd408e03ba6c51719ce100d69bc09b64b36e60`

`operator.yaml` derives from upstream `config/samples/deployment.yaml`. Delivery changes:

- Use SelectDB mirrors for the controller and the 7.3.69 Monitor client initializer.
- Scope the controller and bindings to `fdb-system`; omit optional Node-read RBAC.
- Include only the 7.3 client series used by these samples, for amd64 and arm64.
- Set `IfNotPresent` and explicit initializer resource/security settings.
- Provide separate namespace-scoped Monitor RBAC for unified database Pods.

The optional strategic merge patch adds the amd64-only 7.1.67 client. Apply it
only when using a 7.1 database and after checking the controller node architecture.

These are fresh-install assets. They do not upgrade an existing Operator, database,
or shared CRDs. The legacy `config/operator/fdb-operator.yaml`, root FDB CRDs,
older examples and `helm-charts/doris-foundationdb` remain separate legacy paths;
do not mix them with these versioned assets. Helm migration is not included.
