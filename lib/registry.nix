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

  documents = map (path: builtins.fromJSON (builtins.readFile path)) (walk root);

  entities = builtins.listToAttrs (
    map (document: {
      name = document.metadata.id;
      value = document;
    }) documents
  );

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
    entities
    modelIds
    resolveEntity
    ;
}
