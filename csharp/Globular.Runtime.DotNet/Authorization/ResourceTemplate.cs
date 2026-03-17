// ResourceTemplate — mirrors Go policy.ExpandTemplate().
// Expands {field} placeholders in resource templates with values from request fields.

namespace Globular.Runtime.Authorization;

/// <summary>
/// Utilities for expanding resource path templates.
/// </summary>
public static class ResourceTemplate
{
    /// <summary>
    /// Expands {field} placeholders in a resource template with values from a field map.
    /// Throws <see cref="ResourceTemplateException"/> if a required field is missing or empty.
    /// Returns empty string if the template is null/empty.
    /// </summary>
    /// <example>
    /// ExpandTemplate("/catalog/connections/{connectionId}/items/{itemId}",
    ///     new Dictionary&lt;string, string&gt; { ["connectionId"] = "c-1", ["itemId"] = "i-2" })
    /// → "/catalog/connections/c-1/items/i-2"
    /// </example>
    public static string Expand(string? template, IReadOnlyDictionary<string, string> fields)
    {
        if (string.IsNullOrEmpty(template))
            return "";

        var result = template;
        while (true)
        {
            var start = result.IndexOf('{');
            if (start < 0) break;

            var end = result.IndexOf('}', start);
            if (end < 0)
                throw new ResourceTemplateException($"Unclosed placeholder in template \"{template}\"");

            var fieldName = result[(start + 1)..end];
            if (!fields.TryGetValue(fieldName, out var value) || string.IsNullOrEmpty(value))
                throw new ResourceTemplateException(
                    $"Missing required field \"{fieldName}\" for resource template \"{template}\"");

            result = string.Concat(result.AsSpan(0, start), value, result.AsSpan(end + 1));
        }

        return result;
    }

    /// <summary>
    /// Extracts placeholder field names from a resource template.
    /// </summary>
    public static List<string> ExtractPlaceholders(string template)
    {
        var fields = new List<string>();
        var remaining = template.AsSpan();
        while (true)
        {
            var start = remaining.IndexOf('{');
            if (start < 0) break;

            var end = remaining[start..].IndexOf('}');
            if (end < 0) break;

            fields.Add(remaining[(start + 1)..(start + end)].ToString());
            remaining = remaining[(start + end + 1)..];
        }
        return fields;
    }
}

/// <summary>
/// Thrown when resource template expansion fails due to missing or empty fields.
/// The interceptor catches this and denies the request with a clear error.
/// </summary>
public sealed class ResourceTemplateException : Exception
{
    public ResourceTemplateException(string message) : base(message) { }
}
