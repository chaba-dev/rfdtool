{
  description = "rfdtool: check, link, start, and report on Requests for Discussion";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };

        # The Go release go.mod asks for, used to build and in the dev shell.
        go = pkgs.go_1_26;
        buildGoModule = pkgs.buildGoModule.override { inherit go; };

        rfdtool = buildGoModule {
          pname = "rfdtool";
          version = self.shortRev or self.dirtyShortRev or "dev";
          src = ./.;
          # Update when go.mod or go.sum change: set it to pkgs.lib.fakeHash,
          # run nix build, and copy the hash it reports.
          vendorHash = "sha256-3+raHHPQE20lsZ09Kvljd9FVECCN9q7ZQlKk6KemRig=";
          # The tests read and write git repositories.
          nativeCheckInputs = [ pkgs.git ];
          meta.mainProgram = "rfdtool";
        };
      in
      {
        packages.default = rfdtool;

        apps.default = flake-utils.lib.mkApp { drv = rfdtool; };

        # nix flake check builds the package, which runs go test.
        checks.default = rfdtool;

        devShells.default = pkgs.mkShell {
          packages = [
            go
            pkgs.gopls # language server
            pkgs.gotools # goimports and friends
            pkgs.golangci-lint
            pkgs.delve # debugger
            pkgs.git
          ];
          # Keep the toolchain nix provides rather than downloading the one
          # go.mod names.
          GOTOOLCHAIN = "local";
        };
      }
    );
}
