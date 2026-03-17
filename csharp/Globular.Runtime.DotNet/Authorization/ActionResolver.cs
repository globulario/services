// ActionResolver — mirrors Go policy.ActionResolver.
// Maps gRPC method paths to stable action keys and stores full permission entries.
// Thread-safe via ReaderWriterLockSlim.

namespace Globular.Runtime.Authorization;

/// <summary>
/// Maps gRPC method paths to stable RBAC action keys.
/// Built from permission manifests at service startup; used by interceptors
/// to translate transport-level identifiers to RBAC-level identifiers.
/// </summary>
public sealed class ActionResolver
{
    private readonly ReaderWriterLockSlim _lock = new();
    private readonly Dictionary<string, string> _methodToAction = new();
    private readonly Dictionary<string, PermissionEntry> _methodToPerm = new();
    private readonly Dictionary<string, List<string>> _actionToMethods = new();

    /// <summary>
    /// Registers method→action mappings from a permissions manifest.
    /// </summary>
    public void Register(PermissionsManifest manifest)
    {
        _lock.EnterWriteLock();
        try
        {
            foreach (var perm in manifest.Permissions)
            {
                var method = perm.Method;
                var action = perm.Action;

                if (string.IsNullOrEmpty(method))
                    continue;
                if (string.IsNullOrEmpty(action) || action.StartsWith('/'))
                    continue; // no valid action key

                _methodToAction[method] = action;
                _methodToPerm[method] = perm;

                if (!_actionToMethods.TryGetValue(action, out var methods))
                {
                    methods = new List<string>();
                    _actionToMethods[action] = methods;
                }
                methods.Add(method);
            }
        }
        finally
        {
            _lock.ExitWriteLock();
        }
    }

    /// <summary>
    /// Returns the stable action key for a gRPC method path.
    /// If no mapping exists, returns the method path unchanged (backward compat).
    /// </summary>
    public string Resolve(string method)
    {
        _lock.EnterReadLock();
        try
        {
            return _methodToAction.TryGetValue(method, out var action) ? action : method;
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    /// <summary>
    /// Returns the full PermissionEntry for a gRPC method, or null if unmapped.
    /// </summary>
    public PermissionEntry? ResolvePermission(string method)
    {
        _lock.EnterReadLock();
        try
        {
            return _methodToPerm.TryGetValue(method, out var perm) ? perm : null;
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    /// <summary>
    /// Returns true if a method→action mapping exists.
    /// </summary>
    public bool HasMapping(string method)
    {
        _lock.EnterReadLock();
        try
        {
            return _methodToAction.ContainsKey(method);
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    /// <summary>
    /// Returns the gRPC method paths that map to a given action key.
    /// Used by the migration compatibility shim.
    /// </summary>
    public IReadOnlyList<string>? LegacyMethods(string actionKey)
    {
        _lock.EnterReadLock();
        try
        {
            return _actionToMethods.TryGetValue(actionKey, out var methods)
                ? methods.AsReadOnly()
                : null;
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }
}
