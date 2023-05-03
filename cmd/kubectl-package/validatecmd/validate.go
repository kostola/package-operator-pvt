package validatecmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/package-operator/internal/cmd"
)

func NewCmd() *cobra.Command {
	const (
		validateUse   = "validate [--pull] target"
		validateShort = "validate a package."
		validateLong  = "validate a package. Target may be a source directory, a package in a tar[.gz] or a fully qualified tag if --pull is set."
	)

	cmd := &cobra.Command{
		Use:   validateUse,
		Short: validateShort,
		Long:  validateLong,
		Args:  cobra.ExactArgs(1),
	}

	var opts options

	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		src := args[0]
		if src == "" {
			return fmt.Errorf("%w: 'target' must not be empty", internalcmd.ErrInvalidArgs)
		}

		var validateOptions []internalcmd.ValidatePackageOption

		if opts.Pull {
			validateOptions = append(validateOptions, internalcmd.WithRemoteReference(src))
		} else {
			validateOptions = append(validateOptions, internalcmd.WithPath(src))
		}

		validate := internalcmd.NewValidate()

		if err := validate.ValidatePackage(cmd.Context(), validateOptions...); err != nil {
			return fmt.Errorf("validating package: %w", err)
		}

		return nil
	}

	return cmd
}

type options struct {
	Pull bool
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(
		&o.Pull,
		"pull",
		o.Pull,
		"treat target as image reference and pull it instead of looking on the filesystem",
	)
}