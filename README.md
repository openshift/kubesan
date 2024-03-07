# The Subprovisioner CSI Plugin

Subprovisioner is a [CSI] plugin for [Kubernetes] that enables you to provision
`Block` volumes backed by a single, cluster-wide, shared block device (*e.g.*, a
single big LUN on a SAN).

### Documentation

1. [Getting started](docs/1-getting-started.md)
2. [Architecture](docs/2-architecture.md)
3. [Development](docs/3-development.md)

### Reporting issues

[Create an issue] on GitLab or send an email to afaria@redhat.com.

### License

This project is released under the Apache 2.0 license. See [LICENSE](LICENSE).

[Create an issue]: https://gitlab.com/subprovisioner/subprovisioner/-/issues
[CSI]: https://github.com/container-storage-interface/spec
[Kubernetes]: https://kubernetes.io/
