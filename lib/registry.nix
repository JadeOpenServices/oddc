{ lib }:

let
  root = ../catalog/entities;

  walk =
    dir:
    let
      entries = builtins.readDir dir;
    in
    lib.concatMap (
      name:
      let
        kind = entries.${name};
        path = dir + "/${name}";
      in
      if kind == "directory" then
        walk path
      else if kind == "regular" && lib.hasSuffix ".json" name then
        [ path ]
      else
        [ ]
    ) (builtins.attrNames entries);

  files = map (path: {
    inherit path;
    relative = lib.removePrefix "${toString root}/" (toString path);
    document = builtins.fromJSON (builtins.readFile path);
  }) (walk root);

  entities = builtins.listToAttrs (
    map (file: {
      name = file.document.metadata.id;
      value = file.document;
    }) files
  );

  # Entity ID -> { path, relative } of its canonical file.
  entityFiles = builtins.listToAttrs (
    map (file: {
      name = file.document.metadata.id;
      value = { inherit (file) path relative; };
    }) files
  );

  refsOf =
    value:
    if builtins.isAttrs value then
      (if value ? ref then [ value.ref ] else [ ])
      ++ lib.concatMap refsOf (builtins.attrValues (builtins.removeAttrs value [ "ref" ]))
    else
      [ ];

  # Every entity ID one entity depends on, itself included.
  closure =
    id:
    let
      step =
        seen: pending:
        if pending == [ ] then
          seen
        else
          let
            next = builtins.head pending;
            rest = builtins.tail pending;
          in
          if builtins.elem next seen then
            step seen rest
          else
            step (seen ++ [ next ]) (rest ++ refsOf (builtins.getAttr next entities).data);
    in
    step [ ] [ id ];

  evidenceRoot = ../evidence;

  # Evidence records about one model: list of { path, relative }.
  evidenceFor =
    id:
    if !builtins.pathExists evidenceRoot then
      [ ]
    else
      lib.concatMap (
        path:
        let
          document = builtins.fromJSON (builtins.readFile path);
        in
        lib.optional (document.deviceId or null == id) {
          inherit path;
          relative = lib.removePrefix "${toString evidenceRoot}/" (toString path);
        }
      ) (walk evidenceRoot);

  resolveValue =
    active: value:
    if builtins.isAttrs value then
      if value ? ref then
        let
          ref = value.ref;

          target =
            if builtins.elem ref active then
              throw "ODDC entity reference cycle through ${ref}"
            else if builtins.hasAttr ref entities then
              builtins.getAttr ref entities
            else
              throw "ODDC entity ${builtins.head active} references missing entity ${ref}";

          base = resolveValue (active ++ [ ref ]) target.data;

          local = resolveValue active (builtins.removeAttrs value [ "ref" ]);

          baseWithIdentity = base // {
            id = target.metadata.id;
            name = target.metadata.name;
            kind = target.kind;
          };
        in
        lib.recursiveUpdate baseWithIdentity local
      else
        lib.mapAttrs (_: child: resolveValue active child) value
    else
      value;

  resolveEntity =
    id:
    let
      entity =
        if builtins.hasAttr id entities then
          builtins.getAttr id entities
        else
          throw "Unknown ODDC entity: ${id}";
    in
    {
      inherit (entity) kind;

      id = entity.metadata.id;
      name = entity.metadata.name;
      resolved = resolveValue [ id ] entity.data;
    };

  modelIds = builtins.filter (id: lib.hasPrefix "model/" id) (builtins.attrNames entities);
in
{
  inherit
    closure
    entities
    entityFiles
    evidenceFor
    modelIds
    resolveEntity
    ;
}
