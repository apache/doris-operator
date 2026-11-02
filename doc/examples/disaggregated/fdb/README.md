# FoundationDB installation examples

For a new installation, use the [versioned v2.35.0 guide](v2.35.0/README.md).
It pairs the CRDs and controller from FoundationDB Operator v2.35.0 with the
SelectDB/VeloDB mirrors of the unified FoundationDB Monitor 7.3.69 image.

The YAML files directly in this directory, `config/operator/fdb-operator.yaml`,
`config/crd/bases/apps.foundationdb.org_*` and `helm-charts/doris-foundationdb`
are legacy installation paths with different versions and image modes. Do not
combine their CRDs, controller, client initializers or cluster examples with the
versioned installation. These new-install examples are not an upgrade procedure.
