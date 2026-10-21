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
    let
      # rfdtool built with the Go release go.mod asks for, from any nixpkgs
      # that has it.
      mkRfdtool =
        pkgs:
        let
          version = self.shortRev or self.dirtyShortRev or "dev";
          buildGoModule = pkgs.buildGoModule.override { go = pkgs.go_1_26; };
        in
        buildGoModule {
          pname = "rfdtool";
          inherit version;
          src = self;
          # Update when go.mod or go.sum change: set it to pkgs.lib.fakeHash,
          # run nix build, and copy the hash it reports.
          vendorHash = "sha256-3+raHHPQE20lsZ09Kvljd9FVECCN9q7ZQlKk6KemRig=";
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          # The tests read and write git repositories.
          nativeCheckInputs = [ pkgs.git ];
          meta = {
            description = "Check, link, start, and report on Requests for Discussion";
            homepage = "https://github.com/chaba-dev/rfdtool";
            mainProgram = "rfdtool";
          };
        };
    in
    {
      # Adds pkgs.rfdtool, for flakes that build their own nixpkgs.
      overlays.default = final: prev: { rfdtool = mkRfdtool final; };
    }
    // flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        go = pkgs.go_1_26;
        rfdtool = mkRfdtool pkgs;
      in
      {
        packages = {
          inherit rfdtool;
          default = rfdtool;
        };

        # nix flake check builds the package, which runs go test.
        checks.default = rfdtool;

        # For working on rfdtool itself, with the Go release go.mod asks for.
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
