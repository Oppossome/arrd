{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = with pkgs; [
    go
    gopls
    golangci-lint
    nodejs_22
  ];
}
