{ lib, buildGoModule }:
buildGoModule {
  pname = "oddcctl";
  version = "0.1.0";
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      (lib.fileset.fileFilter (file: file.hasExt "go") ./.)
      ./catalog
      ./evidence
      ./schemas
    ];
  };
  vendorHash = null;
  subPackages = [ "cmd/oddcctl" ];
  # The registry tests read the catalog from the source root.
  doCheck = true;
  postInstall = ''
    mkdir -p $out/share/oddc
    cp -r catalog evidence schemas $out/share/oddc/
  '';
  meta = {
    description = "Validate, resolve and explain ODDC hardware entities";
    license = lib.licenses.asl20;
    mainProgram = "oddcctl";
  };
}
