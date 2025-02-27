# buildx bake

```
docker buildx bake [OPTIONS] [TARGET...]
```

<!---MARKER_GEN_START-->
Build from a file

### Aliases

`bake`, `f`

### Options

| Name | Description |
| --- | --- |
| `--builder string` | Override the configured builder instance |
| [`-f`](#file), [`--file stringArray`](#file) | Build definition file |
| `--load` | Shorthand for --set=*.output=type=docker |
| `--metadata-file string` | Write build result metadata to the file |
| [`--no-cache`](#no-cache) | Do not use cache when building the image |
| [`--print`](#print) | Print the options without building |
| [`--progress string`](#progress) | Set type of progress output (auto, plain, tty). Use plain to show container output |
| [`--pull`](#pull) | Always attempt to pull a newer version of the image |
| `--push` | Shorthand for --set=*.output=type=registry |
| [`--set stringArray`](#set) | Override target value (eg: targetpattern.key=value) |


<!---MARKER_GEN_END-->

## Description

Bake is a high-level build command. Each specified target will run in parallel
as part of the build.

Read [High-level build options](https://github.com/docker/buildx#high-level-build-options) for introduction.

Please note that `buildx bake` command may receive backwards incompatible features in the future if needed. We are looking for feedback on improving the command and extending the functionality further.

## Examples

### <a name="file"></a> Specify a build definition file (-f, --file)

By default, `buildx bake` looks for build definition files in the current directory,
the following are parsed:

- `docker-compose.yml`
- `docker-compose.yaml`
- `docker-bake.json`
- `docker-bake.override.json`
- `docker-bake.hcl`
- `docker-bake.override.hcl`

Use the `-f` / `--file` option to specify the build definition file to use. The
file can be a Docker Compose, JSON or HCL file. If multiple files are specified
they are all read and configurations are combined.

The following example uses a Docker Compose file named `docker-compose.dev.yaml`
as build definition file, and builds all targets in the file:

```console
$ docker buildx bake -f docker-compose.dev.yaml

[+] Building 66.3s (30/30) FINISHED
 => [frontend internal] load build definition from Dockerfile  0.1s
 => => transferring dockerfile: 36B                            0.0s
 => [backend internal] load build definition from Dockerfile   0.2s
 => => transferring dockerfile: 3.73kB                         0.0s
 => [database internal] load build definition from Dockerfile  0.1s
 => => transferring dockerfile: 5.77kB                         0.0s
 ...
```

Pass the names of the targets to build, to build only specific target(s). The
following example builds the `backend` and `database` targets that are defined
in the `docker-compose.dev.yaml` file, skipping the build for the `frontend`
target:

```console
$ docker buildx bake -f docker-compose.dev.yaml backend database

[+] Building 2.4s (13/13) FINISHED
 => [backend internal] load build definition from Dockerfile  0.1s
 => => transferring dockerfile: 81B                           0.0s
 => [database internal] load build definition from Dockerfile 0.2s
 => => transferring dockerfile: 36B                           0.0s
 => [backend internal] load .dockerignore                     0.3s
 ...
```

### <a name="no-cache"></a> Do not use cache when building the image (--no-cache)

Same as `build --no-cache`. Do not use cache when building the image.

### <a name="print"></a> Print the options without building (--print)

Prints the resulting options of the targets desired to be built, in a JSON format,
without starting a build.

```console
$ docker buildx bake -f docker-bake.hcl --print db
{
   "target": {
      "db": {
         "context": "./",
         "dockerfile": "Dockerfile",
         "tags": [
            "docker.io/tiborvass/db"
         ]
      }
   }
}
```

### <a name="progress"></a> Set type of progress output (--progress)

Same as [`build --progress`](buildx_build.md#progress). Set type of progress
output (auto, plain, tty). Use plain to show container output (default "auto").

> You can also use the `BUILDKIT_PROGRESS` environment variable to set its value.

The following example uses `plain` output during the build:

```console
$ docker buildx bake --progress=plain

#2 [backend internal] load build definition from Dockerfile.test
#2 sha256:de70cb0bb6ed8044f7b9b1b53b67f624e2ccfb93d96bb48b70c1fba562489618
#2 ...

#1 [database internal] load build definition from Dockerfile.test
#1 sha256:453cb50abd941762900a1212657a35fc4aad107f5d180b0ee9d93d6b74481bce
#1 transferring dockerfile: 36B done
#1 DONE 0.1s
...
```


### <a name="pull"></a> Always attempt to pull a newer version of the image (--pull)

Same as `build --pull`.

### <a name="set"></a> Override target configurations from command line (--set)

```
--set targetpattern.key[.subkey]=value
```

Override target configurations from command line. The pattern matching syntax is
defined in https://golang.org/pkg/path/#Match.

**Examples**

```console
$ docker buildx bake --set target.args.mybuildarg=value
$ docker buildx bake --set target.platform=linux/arm64
$ docker buildx bake --set foo*.args.mybuildarg=value # overrides build arg for all targets starting with 'foo'
$ docker buildx bake --set *.platform=linux/arm64     # overrides platform for all targets
$ docker buildx bake --set foo*.no-cache              # bypass caching only for targets starting with 'foo'
```

Complete list of overridable fields:
args, cache-from, cache-to, context, dockerfile, labels, no-cache, output, platform,
pull, secrets, ssh, tags, target

### File definition

In addition to compose files, bake supports a JSON and an equivalent HCL file
format for defining build groups and targets.

A target reflects a single docker build invocation with the same options that
you would specify for `docker build`. A group is a grouping of targets.

Multiple files can include the same target and final build options will be
determined by merging them together.

In the case of compose files, each service corresponds to a target.

A group can specify its list of targets with the `targets` option. A target can
inherit build options by setting the `inherits` option to the list of targets or
groups to inherit from.

Note: Design of bake command is work in progress, the user experience may change
based on feedback.


**Example HCL definition**

```hcl
group "default" {
    targets = ["db", "webapp-dev"]
}

target "webapp-dev" {
    dockerfile = "Dockerfile.webapp"
    tags = ["docker.io/username/webapp"]
}

target "webapp-release" {
    inherits = ["webapp-dev"]
    platforms = ["linux/amd64", "linux/arm64"]
}

target "db" {
    dockerfile = "Dockerfile.db"
    tags = ["docker.io/username/db"]
}
```

Complete list of valid target fields:

`args`, `cache-from`, `cache-to`, `context`, `dockerfile`, `inherits`, `labels`,
`no-cache`, `output`, `platform`, `pull`, `secrets`, `ssh`, `tags`, `target`

### HCL variables and functions

Similar to how Terraform provides a way to [define variables](https://www.terraform.io/docs/configuration/variables.html#declaring-an-input-variable),
the HCL file format also supports variable block definitions. These can be used
to define variables with values provided by the current environment, or a default
value when unset.


Example of using interpolation to tag an image with the git sha:

```console
$ cat <<'EOF' > docker-bake.hcl
variable "TAG" {
    default = "latest"
}

group "default" {
    targets = ["webapp"]
}

target "webapp" {
    tags = ["docker.io/username/webapp:${TAG}"]
}
EOF

$ docker buildx bake --print webapp
{
   "target": {
      "webapp": {
         "context": ".",
         "dockerfile": "Dockerfile",
         "tags": [
            "docker.io/username/webapp:latest"
         ]
      }
   }
}

$ TAG=$(git rev-parse --short HEAD) docker buildx bake --print webapp
{
   "target": {
      "webapp": {
         "context": ".",
         "dockerfile": "Dockerfile",
         "tags": [
            "docker.io/username/webapp:985e9e9"
         ]
      }
   }
}
```


A [set of generally useful functions](https://github.com/docker/buildx/blob/master/bake/hclparser/stdlib.go)
provided by [go-cty](https://github.com/zclconf/go-cty/tree/main/cty/function/stdlib)
are available for use in HCL files. In addition, [user defined functions](https://github.com/hashicorp/hcl/tree/main/ext/userfunc)
are also supported.

Example of using the `add` function:

```console
$ cat <<'EOF' > docker-bake.hcl
variable "TAG" {
    default = "latest"
}

group "default" {
    targets = ["webapp"]
}

target "webapp" {
    args = {
        buildno = "${add(123, 1)}"
    }
}
EOF

$ docker buildx bake --print webapp
{
   "target": {
      "webapp": {
         "context": ".",
         "dockerfile": "Dockerfile",
         "args": {
            "buildno": "124"
         }
      }
   }
}
```

Example of defining an `increment` function:

```console
$ cat <<'EOF' > docker-bake.hcl
function "increment" {
    params = [number]
    result = number + 1
}

group "default" {
    targets = ["webapp"]
}

target "webapp" {
    args = {
        buildno = "${increment(123)}"
    }
}
EOF

$ docker buildx bake --print webapp
{
   "target": {
      "webapp": {
         "context": ".",
         "dockerfile": "Dockerfile",
         "args": {
            "buildno": "124"
         }
      }
   }
}
```

Example of only adding tags if a variable is not empty using an `notequal`
function:

```console
$ cat <<'EOF' > docker-bake.hcl
variable "TAG" {default="" }

group "default" {
    targets = [
        "webapp",
    ]
}

target "webapp" {
    context="."
    dockerfile="Dockerfile"
    tags = [
        "my-image:latest",
        notequal("",TAG) ? "my-image:${TAG}": "",
    ]
}
EOF

$ docker buildx bake --print webapp
{
   "target": {
      "webapp": {
         "context": ".",
         "dockerfile": "Dockerfile",
         "tags": [
            "my-image:latest"
         ]
      }
   }
}
```
