{
  lib,
  buildGoModule,
  git,
}:
buildGoModule {
  pname = "oddc";
  version = "0.1.0";
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      (lib.fileset.fileFilter (file: file.hasExt "go") ./.)
      ./catalog
      ./evidence
      ./schemas
      # The update tests take the oddc input from the README.
      ./README.md
    ];
  };
  vendorHash = null;
  subPackages = [ "cmd/oddc" ];
  # The tests read the catalog from the source root.
  doCheck = true;
  # validate --since compares revisions with git.
  nativeCheckInputs = [ git ];
  meta = {
    description = "Validate, resolve and explain ODDC hardware entities";
    license = lib.licenses.asl20;
    mainProgram = "oddc";
  };
}
