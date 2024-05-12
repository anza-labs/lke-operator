# lke-operator

- [lke-operator](#lke-operator)
  - [Overview](#overview)
  - [Description](#description)
  - [Contributing](#contributing)
    - [Development](#development)
  - [Documentation](#documentation)
    - [Developing](#developing)
      - [Setting Up Environment](#setting-up-environment)
      - [Code Formatting and Linting](#code-formatting-and-linting)
      - [Previewing Documentation Locally](#previewing-documentation-locally)
    - [Publishing](#publishing)
      - [Automated Release from main branch](#automated-release-from-main-branch)
      - [Manual Release from Main Branch](#manual-release-from-main-branch)
  - [License](#license)

## Overview
The lke-operator is a Kubernetes operator designed to manage Linode Kubernetes Engine (LKE) clusters. It automates the provisioning, scaling, and management of LKE clusters, simplifying the deployment and maintenance process for Kubernetes workloads on Linode's infrastructure.

## Description
The lke-operator streamlines the deployment and management of LKE clusters. It allows users to define their desired LKE cluster configuration using Kubernetes custom resources, which are then reconciled by the operator to ensure the actual cluster matches the desired state.


## Contributing

### Development

The development of the lke-operator requires the following tools:
- `go`
- `make`
- `docker`

Running any `make` target will install any additional necessary tools required by that target if missing.

## Documentation

### Developing

#### Setting Up Environment

Ensure you have Python and Poetry installed on your system.

```sh
poetry install
```

#### Code Formatting and Linting

To ensure code consistency and quality, use the following commands:

```sh
poetry run black .
poetry run isort --profile=black .
poetry run mypy .
```

#### Previewing Documentation Locally

To preview the documentation locally, run the following command:

```sh
poetry run mkdocs serve
```

### Publishing

#### Automated Release from main branch

Each commit to the `main` branch is automatically released to the `main` tag on the page.

#### Manual Release from Main Branch

To manually release from the `main` branch, follow these steps:

```sh
poetry run publish
```

## License

Copyright 2024 lke-operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
