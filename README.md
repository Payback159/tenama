# temporary namespace manager (tenama)

Tenama provides a simple REST API that allows non-cluster administrators in a shared Kubernetes environment to create temporary namespaces. tenama handles the creation, management, and cleanup of the temporary namespaces.

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/Payback159/tenama/badge)](https://api.securityscorecards.dev/projects/github.com/Payback159/tenama)

## Running the server

```bash
nerdctl build . -t tenama
nerdctl run --rm -p 8080:8080 -v ./config.yaml:config.yaml tenama
```

## Create Namespace Sequence-Diagram

[<img src="./.docs/diagramms/createNamespaceSeq.png">]()

## Cleanup Namespaces Sequence-Diagram

[<img src="./.docs/diagramms/cleanupNamespaces.png">]()

test