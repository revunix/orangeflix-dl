{
  description = "Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "1.1.2";
      repo = "revunix/orangeflix-dl";
      systems = {
        x86_64-linux = { os = "linux"; arch = "amd64"; hash = "sha256-4WgcKS9/wF3RwrCTqZaQrHNNyYDeYVkQoKg7DrHay5c="; };
        aarch64-linux = { os = "linux"; arch = "arm64"; hash = "sha256-ezWKKw+NkU3hGG4WmH6jeIfZbYu3q6CbPLnVkF4MXYI="; };
        x86_64-darwin = { os = "darwin"; arch = "amd64"; hash = "sha256-VZ/4WnvOS6DvOln5Owl0cnpGS4GqMtn6DFNsOL7mQGE="; };
        aarch64-darwin = { os = "darwin"; arch = "arm64"; hash = "sha256-H2SJ1VC/OLy1sEWsw9Kpsryi9h+66CoArX1JASldJ20="; };
      };
      each = nixpkgs.lib.genAttrs (builtins.attrNames systems);
    in
    {
      packages = each (system:
        let
          pkgs = import nixpkgs { inherit system; };
          spec = systems.${system};
          archive = "orangeflix-dl_v${version}_${spec.os}_${spec.arch}.tar.gz";
        in
        {
          default = pkgs.stdenvNoCC.mkDerivation {
            pname = "orangeflix-dl";
            inherit version;
            src = pkgs.fetchurl {
              url = "https://github.com/${repo}/releases/download/v${version}/${archive}";
              hash = spec.hash;
            };
            nativeBuildInputs = [ pkgs.installShellFiles ];
            dontUnpack = true;
            installPhase = ''
              runHook preInstall
              tar -xzf $src
              install -Dm755 orangeflix-dl $out/bin/orangeflix-dl
              installShellCompletion --cmd orangeflix-dl \
                --bash <($out/bin/orangeflix-dl --completions bash) \
                --zsh <($out/bin/orangeflix-dl --completions zsh) \
                --fish <($out/bin/orangeflix-dl --completions fish)
              $out/bin/orangeflix-dl --man > orangeflix-dl.1
              installManPage orangeflix-dl.1
              runHook postInstall
            '';
          };
        });
    };
}
