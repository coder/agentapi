{
  description = "Devshell for ACP Go SDK";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    flake-utils.url = "github:numtide/flake-utils";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      treefmt-nix,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        treefmtEval = treefmt-nix.lib.evalModule pkgs {
          projectRootFile = "flake.nix";
          programs = {
            actionlint.enable = true;
            gofmt.enable = true;
            mdformat.enable = true;
            mdsh.enable = true;
            nixfmt.enable = true;
            zizmor.enable = true;
          };
        };
        formatter = treefmtEval.config.build.wrapper;
      in
      {
        inherit formatter;

        checks = {
          formatting = treefmtEval.config.build.check self;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            # Go toolchain and editor helpers
            go_1_24
            gopls
            golangci-lint

            # Build and release tooling
            git
            gnumake
            curl

            # Misc developer conveniences
            mdsh

            # Tree-wide formatter wrapper (treefmt)
            formatter
          ];
        };
      }
    );
}
