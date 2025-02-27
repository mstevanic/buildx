package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/util/buildflags"
	"github.com/docker/buildx/util/platformutil"
	"github.com/docker/buildx/util/progress"
	"github.com/docker/buildx/util/tracing"
	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const defaultTargetName = "default"

type buildOptions struct {
	commonOptions
	contextPath    string
	dockerfileName string
	tags           []string
	labels         []string
	buildArgs      []string

	cacheFrom   []string
	cacheTo     []string
	target      string
	platforms   []string
	secrets     []string
	ssh         []string
	outputs     []string
	imageIDFile string
	extraHosts  []string
	networkMode string

	// unimplemented
	squash bool
	quiet  bool

	allow []string

	// hidden
	// untrusted   bool
	// ulimits        *opts.UlimitOpt
	// memory         opts.MemBytes
	// memorySwap     opts.MemSwapBytes
	// shmSize        opts.MemBytes
	// cpuShares      int64
	// cpuPeriod      int64
	// cpuQuota       int64
	// cpuSetCpus     string
	// cpuSetMems     string
	// cgroupParent   string
	// isolation      string
	// compress    bool
	// securityOpt []string
}

type commonOptions struct {
	builder      string
	noCache      *bool
	progress     string
	pull         *bool
	metadataFile string
	// golangci-lint#826
	// nolint:structcheck
	exportPush bool
	// nolint:structcheck
	exportLoad bool
}

func runBuild(dockerCli command.Cli, in buildOptions) (err error) {
	if in.squash {
		return errors.Errorf("squash currently not implemented")
	}
	if in.quiet {
		logrus.Warnf("quiet currently not implemented")
	}

	ctx := appcontext.Context()

	ctx, end, err := tracing.TraceCurrentCommand(ctx, "build")
	if err != nil {
		return err
	}
	defer func() {
		end(err)
	}()

	noCache := false
	if in.noCache != nil {
		noCache = *in.noCache
	}
	pull := false
	if in.pull != nil {
		pull = *in.pull
	}

	opts := build.Options{
		Inputs: build.Inputs{
			ContextPath:    in.contextPath,
			DockerfilePath: in.dockerfileName,
			InStream:       os.Stdin,
		},
		Tags:        in.tags,
		Labels:      listToMap(in.labels, false),
		BuildArgs:   listToMap(in.buildArgs, true),
		Pull:        pull,
		NoCache:     noCache,
		Target:      in.target,
		ImageIDFile: in.imageIDFile,
		ExtraHosts:  in.extraHosts,
		NetworkMode: in.networkMode,
	}

	platforms, err := platformutil.Parse(in.platforms)
	if err != nil {
		return err
	}
	opts.Platforms = platforms

	opts.Session = append(opts.Session, authprovider.NewDockerAuthProvider(os.Stderr))

	secrets, err := buildflags.ParseSecretSpecs(in.secrets)
	if err != nil {
		return err
	}
	opts.Session = append(opts.Session, secrets)

	sshSpecs := in.ssh
	if len(sshSpecs) == 0 && buildflags.IsGitSSH(in.contextPath) {
		sshSpecs = []string{"default"}
	}
	ssh, err := buildflags.ParseSSHSpecs(sshSpecs)
	if err != nil {
		return err
	}
	opts.Session = append(opts.Session, ssh)

	outputs, err := buildflags.ParseOutputs(in.outputs)
	if err != nil {
		return err
	}
	if in.exportPush {
		if in.exportLoad {
			return errors.Errorf("push and load may not be set together at the moment")
		}
		if len(outputs) == 0 {
			outputs = []client.ExportEntry{{
				Type: "image",
				Attrs: map[string]string{
					"push": "true",
				},
			}}
		} else {
			switch outputs[0].Type {
			case "image":
				outputs[0].Attrs["push"] = "true"
			default:
				return errors.Errorf("push and %q output can't be used together", outputs[0].Type)
			}
		}
	}
	if in.exportLoad {
		if len(outputs) == 0 {
			outputs = []client.ExportEntry{{
				Type:  "docker",
				Attrs: map[string]string{},
			}}
		} else {
			switch outputs[0].Type {
			case "docker":
			default:
				return errors.Errorf("load and %q output can't be used together", outputs[0].Type)
			}
		}
	}

	opts.Exports = outputs

	cacheImports, err := buildflags.ParseCacheEntry(in.cacheFrom)
	if err != nil {
		return err
	}
	opts.CacheFrom = cacheImports

	cacheExports, err := buildflags.ParseCacheEntry(in.cacheTo)
	if err != nil {
		return err
	}
	opts.CacheTo = cacheExports

	allow, err := buildflags.ParseEntitlements(in.allow)
	if err != nil {
		return err
	}
	opts.Allow = allow

	// key string used for kubernetes "sticky" mode
	contextPathHash, err := filepath.Abs(in.contextPath)
	if err != nil {
		contextPathHash = in.contextPath
	}

	return buildTargets(ctx, dockerCli, map[string]build.Options{defaultTargetName: opts}, in.progress, contextPathHash, in.builder, in.metadataFile)
}

func buildTargets(ctx context.Context, dockerCli command.Cli, opts map[string]build.Options, progressMode, contextPathHash, instance string, metadataFile string) error {
	dis, err := getInstanceOrDefault(ctx, dockerCli, instance, contextPathHash)
	if err != nil {
		return err
	}

	ctx2, cancel := context.WithCancel(context.TODO())
	defer cancel()
	printer := progress.NewPrinter(ctx2, os.Stderr, progressMode)

	resp, err := build.Build(ctx, dis, opts, dockerAPI(dockerCli), dockerCli.ConfigFile(), printer)
	err1 := printer.Wait()
	if err == nil {
		err = err1
	}
	if err != nil {
		return err
	}

	if len(metadataFile) > 0 && resp != nil {
		mdatab, err := json.MarshalIndent(resp[defaultTargetName].ExporterResponse, "", "  ")
		if err != nil {
			return err
		}
		if err := ioutils.AtomicWriteFile(metadataFile, mdatab, 0644); err != nil {
			return err
		}
	}

	return err
}

func buildCmd(dockerCli command.Cli, rootOpts *rootOptions) *cobra.Command {
	var options buildOptions

	cmd := &cobra.Command{
		Use:     "build [OPTIONS] PATH | URL | -",
		Aliases: []string{"b"},
		Short:   "Start a build",
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.contextPath = args[0]
			options.builder = rootOpts.builder
			return runBuild(dockerCli, options)
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&options.exportPush, "push", false, "Shorthand for --output=type=registry")
	flags.BoolVar(&options.exportLoad, "load", false, "Shorthand for --output=type=docker")

	flags.StringArrayVarP(&options.tags, "tag", "t", []string{}, "Name and optionally a tag in the 'name:tag' format")
	flags.SetAnnotation("tag", "docs.external.url", []string{"https://docs.docker.com/engine/reference/commandline/build/#tag-an-image--t"})
	flags.StringArrayVar(&options.buildArgs, "build-arg", []string{}, "Set build-time variables")
	flags.SetAnnotation("build-arg", "docs.external.url", []string{"https://docs.docker.com/engine/reference/commandline/build/#set-build-time-variables---build-arg"})

	flags.StringVarP(&options.dockerfileName, "file", "f", "", "Name of the Dockerfile (Default is 'PATH/Dockerfile')")
	flags.SetAnnotation("file", "docs.external.url", []string{"https://docs.docker.com/engine/reference/commandline/build/#specify-a-dockerfile--f"})

	flags.StringArrayVar(&options.labels, "label", []string{}, "Set metadata for an image")

	flags.StringArrayVar(&options.cacheFrom, "cache-from", []string{}, "External cache sources (eg. user/app:cache, type=local,src=path/to/dir)")
	flags.StringArrayVar(&options.cacheTo, "cache-to", []string{}, "Cache export destinations (eg. user/app:cache, type=local,dest=path/to/dir)")

	flags.StringVar(&options.target, "target", "", "Set the target build stage to build.")
	flags.SetAnnotation("target", "docs.external.url", []string{"https://docs.docker.com/engine/reference/commandline/build/#specifying-target-build-stage---target"})

	flags.StringSliceVar(&options.allow, "allow", []string{}, "Allow extra privileged entitlement, e.g. network.host, security.insecure")

	// not implemented
	flags.BoolVarP(&options.quiet, "quiet", "q", false, "Suppress the build output and print image ID on success")
	flags.StringVar(&options.networkMode, "network", "default", "Set the networking mode for the RUN instructions during build")
	flags.StringSliceVar(&options.extraHosts, "add-host", []string{}, "Add a custom host-to-IP mapping (host:ip)")
	flags.SetAnnotation("add-host", "docs.external.url", []string{"https://docs.docker.com/engine/reference/commandline/build/#add-entries-to-container-hosts-file---add-host"})
	flags.StringVar(&options.imageIDFile, "iidfile", "", "Write the image ID to the file")
	flags.BoolVar(&options.squash, "squash", false, "Squash newly built layers into a single new layer")
	flags.MarkHidden("quiet")
	flags.MarkHidden("squash")

	// hidden flags
	var ignore string
	var ignoreSlice []string
	var ignoreBool bool
	var ignoreInt int64
	flags.StringVar(&ignore, "ulimit", "", "Ulimit options")
	flags.MarkHidden("ulimit")
	flags.StringSliceVar(&ignoreSlice, "security-opt", []string{}, "Security options")
	flags.MarkHidden("security-opt")
	flags.BoolVar(&ignoreBool, "compress", false, "Compress the build context using gzip")
	flags.MarkHidden("compress")
	flags.StringVarP(&ignore, "memory", "m", "", "Memory limit")
	flags.MarkHidden("memory")
	flags.StringVar(&ignore, "memory-swap", "", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flags.MarkHidden("memory-swap")
	flags.StringVar(&ignore, "shm-size", "", "Size of /dev/shm")
	flags.MarkHidden("shm-size")
	flags.Int64VarP(&ignoreInt, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	flags.MarkHidden("cpu-shares")
	flags.Int64Var(&ignoreInt, "cpu-period", 0, "Limit the CPU CFS (Completely Fair Scheduler) period")
	flags.MarkHidden("cpu-period")
	flags.Int64Var(&ignoreInt, "cpu-quota", 0, "Limit the CPU CFS (Completely Fair Scheduler) quota")
	flags.MarkHidden("cpu-quota")
	flags.StringVar(&ignore, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	flags.MarkHidden("cpuset-cpus")
	flags.StringVar(&ignore, "cpuset-mems", "", "MEMs in which to allow execution (0-3, 0,1)")
	flags.MarkHidden("cpuset-mems")
	flags.StringVar(&ignore, "cgroup-parent", "", "Optional parent cgroup for the container")
	flags.MarkHidden("cgroup-parent")
	flags.StringVar(&ignore, "isolation", "", "Container isolation technology")
	flags.MarkHidden("isolation")
	flags.BoolVar(&ignoreBool, "rm", true, "Remove intermediate containers after a successful build")
	flags.MarkHidden("rm")
	flags.BoolVar(&ignoreBool, "force-rm", false, "Always remove intermediate containers")
	flags.MarkHidden("force-rm")

	platformsDefault := []string{}
	if v := os.Getenv("DOCKER_DEFAULT_PLATFORM"); v != "" {
		platformsDefault = []string{v}
	}
	flags.StringArrayVar(&options.platforms, "platform", platformsDefault, "Set target platform for build")

	flags.StringArrayVar(&options.secrets, "secret", []string{}, "Secret file to expose to the build: id=mysecret,src=/local/secret")

	flags.StringArrayVar(&options.ssh, "ssh", []string{}, "SSH agent socket or keys to expose to the build (format: default|<id>[=<socket>|<key>[,<key>]])")

	flags.StringArrayVarP(&options.outputs, "output", "o", []string{}, "Output destination (format: type=local,dest=path)")

	commonBuildFlags(&options.commonOptions, flags)

	return cmd
}

func commonBuildFlags(options *commonOptions, flags *pflag.FlagSet) {
	options.noCache = flags.Bool("no-cache", false, "Do not use cache when building the image")
	flags.StringVar(&options.progress, "progress", "auto", "Set type of progress output (auto, plain, tty). Use plain to show container output")
	options.pull = flags.Bool("pull", false, "Always attempt to pull a newer version of the image")
	flags.StringVar(&options.metadataFile, "metadata-file", "", "Write build result metadata to the file")
}

func listToMap(values []string, defaultEnv bool) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) == 1 {
			if defaultEnv {
				v, ok := os.LookupEnv(kv[0])
				if ok {
					result[kv[0]] = v
				}
			} else {
				result[kv[0]] = ""
			}
		} else {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
