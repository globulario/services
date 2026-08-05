# Reference OCI-backed Globular package

This directory is a complete authoring example for an OCI-backed service.

It intentionally contains no host binary, post-install script, `.deb`, or
systemd unit. The Node Agent validates the package contract and renders the unit
from the node-owned lifecycle layout.

The image reference is illustrative. Replace its digest with an immutable digest
from the selected registry before publishing the package.
