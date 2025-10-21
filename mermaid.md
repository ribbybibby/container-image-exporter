# Container Image Exporter Architecture

## System Overview

```mermaid
graph TB
    subgraph "Kubernetes Cluster"
        POD[Pods]
        DEPLOY[Deployments]
        STS[StatefulSets]
        DS[DaemonSets]
        JOB[Jobs]
        CRON[CronJobs]
    end

    subgraph "Container Image Exporter"
        subgraph "Controllers"
            CTRL1[Pod Controller]
            CTRL2[Deployment Controller]
            CTRL3[StatefulSet Controller]
            CTRL4[DaemonSet Controller]
            CTRL5[Job Controller]
            CTRL6[CronJob Controller]
        end

        RECONCILER[ContainerImageReconciler]
        CACHE[ContainerImageCache<br/>In-Memory Cache<br/>TTL: 1 hour]
        EXPORTER[Prometheus Exporter<br/>:8080/metrics]
    end

    subgraph "External"
        REGISTRY[Container Registry<br/>Docker Hub, GCR, ECR, etc.]
        PROM[Prometheus<br/>Scraper]
    end

    POD --> CTRL1
    DEPLOY --> CTRL2
    STS --> CTRL3
    DS --> CTRL4
    JOB --> CTRL5
    CRON --> CTRL6

    CTRL1 --> RECONCILER
    CTRL2 --> RECONCILER
    CTRL3 --> RECONCILER
    CTRL4 --> RECONCILER
    CTRL5 --> RECONCILER
    CTRL6 --> RECONCILER

    RECONCILER -->|Extract container specs| RECONCILER
    RECONCILER -->|Check cache| CACHE
    CACHE -->|Cache miss/expired| RECONCILER
    RECONCILER -->|Fetch metadata| REGISTRY
    REGISTRY -->|Digest, labels, annotations,<br/>size, created time| RECONCILER
    RECONCILER -->|Store metadata| CACHE

    EXPORTER -->|Query on scrape| CACHE
    EXPORTER -->|List resources| POD
    EXPORTER -->|List resources| DEPLOY
    EXPORTER -->|List resources| STS
    EXPORTER -->|List resources| DS
    EXPORTER -->|List resources| JOB
    EXPORTER -->|List resources| CRON
    PROM -->|Scrape metrics| EXPORTER

    style CACHE fill:#e1f5ff
    style RECONCILER fill:#fff3e1
    style EXPORTER fill:#f3e1ff
    style REGISTRY fill:#ffe1e1
```

## Reconciliation Flow

```mermaid
sequenceDiagram
    participant K8s as Kubernetes API
    participant Ctrl as Controller
    participant Rec as Reconciler
    participant Cache as Image Cache
    participant Reg as Container Registry

    K8s->>Ctrl: Resource Create/Update Event
    Ctrl->>Rec: Reconcile(resource)

    loop For each container in resource
        Rec->>Rec: Extract image reference
        Rec->>Cache: Get(imageRef)

        alt Cache hit & not expired
            Cache-->>Rec: Return cached metadata
        else Cache miss or expired
            Rec->>Rec: Build k8s keychain<br/>(pull secrets, SA tokens)
            Rec->>Reg: Fetch image metadata
            Reg-->>Rec: Digest, annotations,<br/>labels, size, created
            Rec->>Cache: Store(imageRef, metadata)
        end
    end

    Rec-->>Ctrl: Requeue after cache duration + jitter
```

## Metrics Export Flow

```mermaid
sequenceDiagram
    participant Prom as Prometheus
    participant Exp as Exporter
    participant K8s as Kubernetes API
    participant Cache as Image Cache

    Prom->>Exp: GET /metrics

    loop For each resource type
        Exp->>K8s: List(Pods/Deployments/etc.)
        K8s-->>Exp: Resources

        loop For each resource
            loop For each container
                Exp->>Cache: Get(imageRef)
                Cache-->>Exp: Metadata or nil

                alt Metadata exists
                    Exp->>Exp: Emit container_image_container_info
                    Exp->>Exp: Emit container_image_size_bytes
                    Exp->>Exp: Emit container_image_created

                    loop For each annotation
                        Exp->>Exp: Emit container_image_annotation
                    end

                    loop For each label
                        Exp->>Exp: Emit container_image_label
                    end
                end
            end
        end
    end

    Exp-->>Prom: Prometheus metrics
```

## Data Extraction from Kubernetes Resources

```mermaid
graph LR
    subgraph "Resource Types"
        POD[Pod]
        DEPLOY[Deployment/StatefulSet/DaemonSet]
        CRON[CronJob]
        JOB[Job]
    end

    subgraph "Container Specs Extracted"
        INIT[spec.initContainers]
        CONT[spec.containers]
        EPHEM[spec.ephemeralContainers]
        TINIT[spec.template.spec.initContainers]
        TCONT[spec.template.spec.containers]
        JINIT[spec.jobTemplate.spec.template.spec.initContainers]
        JCONT[spec.jobTemplate.spec.template.spec.containers]
    end

    POD --> INIT
    POD --> CONT
    POD --> EPHEM

    DEPLOY --> TINIT
    DEPLOY --> TCONT
    JOB --> TINIT
    JOB --> TCONT

    CRON --> JINIT
    CRON --> JCONT

    INIT --> IMAGE[Image Reference<br/>e.g., nginx:latest]
    CONT --> IMAGE
    EPHEM --> IMAGE
    TINIT --> IMAGE
    TCONT --> IMAGE
    JINIT --> IMAGE
    JCONT --> IMAGE
```

## Exported Metrics

The exporter provides the following Prometheus metrics:

1. **container_image_container_info** (gauge)
   - Labels: `group`, `version`, `kind`, `namespace`, `name`, `jsonpath`, `image`, `digest`
   - Links containers to their image digests

2. **container_image_annotation** (gauge)
   - Labels: `digest`, `key`, `value`
   - Exposes manifest annotations from the image

3. **container_image_label** (gauge)
   - Labels: `digest`, `key`, `value`
   - Exposes config labels from the image

4. **container_image_size_bytes** (gauge)
   - Labels: `digest`
   - Total size of image in the registry (config + all layers)

5. **container_image_created** (gauge)
   - Labels: `digest`
   - Unix timestamp when the image was created

## Authentication Flow

```mermaid
graph TB
    REC[Reconciler]
    CHAIN[k8schain Keychain]

    subgraph "Credential Sources"
        PS[Pull Secrets]
        SA[ServiceAccount Tokens]
        GCP[GCP Metadata Server]
        AWS[AWS IAM/IRSA]
        AZURE[Azure Managed Identity]
        DOCKER[Docker Config]
    end

    REC -->|Build keychain for namespace| CHAIN
    CHAIN -->|Query in order| PS
    CHAIN -->|Query in order| SA
    CHAIN -->|Query in order| GCP
    CHAIN -->|Query in order| AWS
    CHAIN -->|Query in order| AZURE
    CHAIN -->|Query in order| DOCKER

    PS -->|Credentials| REG[Container Registry]
    SA -->|Credentials| REG
    GCP -->|Credentials| REG
    AWS -->|Credentials| REG
    AZURE -->|Credentials| REG
    DOCKER -->|Credentials| REG
```
