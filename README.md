# temporary namespace manager (tenama)

tenama provides a simple REST API to enable non-cluster-admins to create temporary namespaces. tenama takes care of the creation, management and cleanup of the temporary namespaces.

## Overview



### Running the server

```bash
nerdctl build . -t tenama
nerdctl run --rm -p 8080:8080 -v ./config.yaml:config.yaml tenama
```