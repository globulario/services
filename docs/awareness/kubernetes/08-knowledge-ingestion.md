# Kubernetes Awareness — Knowledge Ingestion

## How to Add a New K8s Failure Mode

When a new Kubernetes failure pattern is observed in production:

1. **Classify it** using the taxonomy in `03-failure-taxonomy.md`
2. **Determine the authority boundary** using `04-substrate-boundaries.md`
3. **Add a failure mode entry** in `docs/awareness/failure_modes.yaml`:
   ```yaml
   - id: fm.k8s.<short_name>
     title: <one-line description>
     severity: critical|high|medium|low
     classification: workload_owned|kubernetes_control_plane_owned|substrate_owned
     summary: |
       <what happens, when, and why it matters>
     symptoms:
       - <observable symptom 1>
       - <observable symptom 2>
     root_cause: |
       <substrate or workload root cause>
     architecture_fix: |
       <correct approach>
     known_bad_fixes:
       - <wrong fix that makes things worse>
     related_invariants:
       - k8s.<relevant_invariant>
   ```

4. **Add an invariant** if the failure reveals a rule that must always hold:
   ```yaml
   - id: k8s.<component>.<property>
     title: <what must be true>
     severity: critical|high
     status: active
     summary: |
       <why this must hold and what breaks if it doesn't>
     required_tests:
       - Test<Name>
   ```

5. **Run graph rebuild**: `globular awareness build --clean`

---

## How to Add a New K8s Observation Source

If Globular gains a new way to observe Kubernetes state (new MCP tool, new metrics query, new gRPC call):

1. Document the observation source in `05-observation-sources.md`
2. Add the evidence contract to `docs/awareness/authority_rules.yaml`:
   ```yaml
   - id: authority.k8s.<source_name>
     layer: Runtime
     question: <what question does this source answer?>
     rule: <when to use this source and what it proves>
     correct_authority:
       - <source>
     related_invariants:
       - k8s.<invariant>
   ```
3. Update `detector_mapping.yaml` if it adds new metric-to-invariant mappings

---

## Staleness and Freshness

K8s awareness knowledge has a `stale_after_days: 365` default.

Update invariants when:
- Kubernetes releases a new major version that changes behavior
- A new production incident reveals an undocumented failure pattern
- A previous classification is found to be wrong

Mark a stale entry by updating `last_verified` and adding a note in `summary`.
