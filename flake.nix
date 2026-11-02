{
  description = "Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "1.3.0";
      repo = "revunix/orangeflix-dl";
      systems = {
        x86_64-linux = { os = "linux"; arch = "amd64"; hash = "sha256-SB4/vYg39+CPwlmmkPcjM5EgnXoT2A8Mmmw2jYSoBMo="; };
        aarch64-linux = { os = "linux"; arch = "arm64"; hash = "sha256-C3jJg2zC0n2TC0tgrn4BFY62ats6cD2Abz4MgzG79yk="; };
        x86_64-darwin = { os = "darwin"; arch = "amd64"; hash = "sha256-vgT+27PIoD9AE8l/I0iXILahEgUe7Wf8BNou3xbIZc0="; };
        aarch64-darwin = { os = "darwin"; arch = "arm64"; hash = "sha256-JKF2Os6FJLhB02TxIaHyC+tVsB+Lhohg/VJLXVlw6GY="; };
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
