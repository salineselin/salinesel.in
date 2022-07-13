---
title: "Good Kyverno Admission Control Patterns"
description:
date: 2022-07-12T16:12:18-06:00
image: "pexels-photo-1169754.jpeg"
slug: kyverno-patterns
draft: false
tags:
  - kubernetes
---

Kyverno is an admission controller used to add policies to your cluster. The basic principle of an admission controller is to intercept incoming requests to a given kubernetes api server and check if a field matches an expression, then approve or deny the request based on that determination. If you're not familiar with admission controllers already, I'd recommend reading [the official documentation](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#what-are-they).

If you're in the process of deciding what admisson controller to use in your environment, consider the background of the team that will be maintaining the policies. Do they all know how to program in Golang? If so, [OPA](https://www.openpolicyagent.org/) may be a better choice for you. I personally believe that Kyverno is a better admission controller because the barrier for reading and writing policies is significantly lower than OPA. I haven't run into a scenario where I need more verbose or complex syntax beyond what ships in Kyverno by default.

## THE GIST

- Have some way to whitelist resources
- Make all your transformations with JSON patches
- Narrow down the scope of your whitelists as much as possible
- Configure all policies to accept an array of rules rather than a single ruleset
- Have some way to unit test against admission controller policies when working with k8s manifests in a github repository
- Separate your Kyverno installation from the underlying policies
- Have some way to toggle rule actions between audit and enforcement
- Add remote policies using raw.githubusercontent.com rather than copying them locally
- When referencing remote policies, target a commit hash or branch version rather than the main branch's head
- Avoid mutating resources with policies when possible

## RECOMMENDATIONS

While not required, I highly recommend doing the following

### Have some way to whitelist resources

It is inevitable that you will have resources in your kubernetes clusters that violate policies. Defining some generic templatized process around how you add exceptions to a whitelist is crucial. You can whitelist resources using syntax with exceptions using the `exclude` key like so:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-capabilities
spec:
  validationFailureAction: audit
  background: true
  rules:
    - name: adding-capabilities
      exclude:
        any:
          - resources:
              kinds:
                - Pod
              selector:
                matchLabels:
                  app: managed-prometheus-collector
      match:
        any:
          - resources:
              kinds:
                - Pod
      validate:
        message: >-
          Any capabilities added beyond the allowed list (AUDIT_WRITE, CHOWN, DAC_OVERRIDE, FOWNER,
          FSETID, KILL, MKNOD, NET_BIND_SERVICE, SETFCAP, SETGID, SETPCAP, SETUID, SYS_CHROOT)
          are disallowed.
        deny:
          conditions:
            all:
              - key: "{{ request.object.spec.[ephemeralContainers, initContainers, containers][].securityContext.capabilities.add[] }}"
                operator: AnyNotIn
                value:
                  - SYS_CHROOT
```

### Make all your changes with JSON patches

This applies if you're using Kustomize to manage your Kyverno transformations. Kustomize overlays are excellent at implicitly overlaying all the necessary parameters, but when you start working with array indexes more, you start wiping data that you don't intend to and you usually end up repeating yourself a lot. If you make all your transformations with JSON patches rather than overlays, you have a complete list of all your transformations, and debugging those transformations becomes a lot easier when kustomize executes and can explicitly point out a faulty JSON patch.

### Narrow down the scope of your whitelists as much as possible

Whitelisting a namespace is a very primitive control for adding exceptions for entities. It's fast and easy to understand, but is grossly overpermissive. Unless you have your RBAC hardened to a point where clusters users don't have visibility into what the policy exceptions are, its trivial for an attacker to just use a different namespace that's been whitelisted.

### Configure all policies to accept an array of rules rather than a single ruleset

There are multiple valid syntaxes when defining a policy. You can match according to one object or an array of objects. The preferred method is an array of objects. It's not mentioned in Kyverno's documentation, but if you use an `any` match selector, you also have to use the `any` selector when creating exclusions. To demonstrate why you want to only work with an array of matches rather than a single defined match, let's work with the following policy:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-capabilities
spec:
  validationFailureAction: audit
  background: true
  rules:
    - name: adding-capabilities
      match:
        resources:
          kinds:
            - Pod
      validate:
        message: >-
          Any capabilities added beyond the allowed list (AUDIT_WRITE, CHOWN, DAC_OVERRIDE, FOWNER,
          FSETID, KILL, MKNOD, NET_BIND_SERVICE, SETFCAP, SETGID, SETPCAP, SETUID, SYS_CHROOT)
          are disallowed.
        deny:
          conditions:
            all:
              - key: "{{ request.object.spec.[ephemeralContainers, initContainers, containers][].securityContext.capabilities.add[] }}"
                operator: AnyNotIn
                value:
                  - SYS_CHROOT
```

This policy which you can find in source [here](https://github.com/kyverno/policies/blob/main/pod-security/baseline/disallow-capabilities/disallow-capabilities.yaml) is perfectly valid. The problem is when you want to whitelist a particular namespace or add another ruleset with more advanced targeting, you have to heavily modify the underlying ruleset with JSON patches.

Let's say I wanted to add two exceptions according to a label selector. Instead of a single JSON patch that looks something like the following, you would have to add multiple patches to get your policy into your defined state. Transformations are necessary, but excessive transformations mean you have more code to maintain, and legibility is decreased.

```yaml
- op: add
  path: /spec/rules/0/exclude
  value:
    any:
      - resources:
          kinds:
            - Pod
          selector:
            matchLabels:
              # gke's managed prometheus uses the host ports
              app: managed-prometheus-collector
      - resources:
          kinds:
            - Pod
          selector:
            matchLabels:
              # gke's managed prometheus rule evaluator needs them as well
              app: rule-evaluator
```

This is the desirable syntax

```yaml
match:
  any:
    - resources:
        kinds:
          - Pod
```

And this is the less-than-desirable syntax

```yaml
match:
  resources:
    kinds:
      - Pod
```

Almost all of the policies you find today in the [public Kyverno policies repository](https://github.com/kyverno/policies) now use the array syntax by default, but it wasn't always that way. In [this earlier commit](https://github.com/kyverno/policies/commit/944498575e423eb98d4b31f6ae69e1e8161004c6#diff-77b3af0188557903b284d66fac9ef6be3532b5474ec9382c28f1ec8388f832d1) you can find remnants of when the configured rules did not use the array syntax.

### Have some way to unit test against admission controller policies when working with k8s manifests in a github repository

Debugging CI sucks. If your CI|CD pipeline is verbose and takes five minutes to deploy something to a cluster, but it fails due to a failed policy check, it chisels at your soul. Feedback loops become repetitive, slow, and unfruitful. If you're using a CI provider like Github Actions, make a pipeline that runs a unit test using `kubectl apply -f /path/to/yaml --dry-run=server`

If that isn't soon enough and you're still experiencing pain with iteration, you could potentially use a git hook when you push source control changes to remote (similar to how [husky](https://www.npmjs.com/package/husky) does it) to get that feedback even sooner. I'd recommend only implementing a hook like this if the developers working on kubernetes manifests are acclimated to kubernetes and have their policies pulled down into their local dev cluster, or they're authenticated into a remote cluster with a context they can use to `--dry-run=server` against.

Truth be told most of the manifests you write are likely written and then rarely touched, so iteration slow and frequent enough to cause admission controller heartache is likely seldom.

### Separate your Kyverno installation from the underlying policies

Respect that the policies you install are separate from the application that enforces the policies. They have divergent lifecycles, but should be respected as harmonious companions

### Have some way to toggle rule actions between audit and enforcement

If you're working with remote policies a lot, most of them are usually set to `audit` rather than `enforce`, so you'll need to make a transformation that changes the `validationFailureAction` value. I use this JSON patch:

```yaml
- op: replace
  path: /spec/validationFailureAction
  value: enforce
```

### Add remote policies using raw.githubusercontent.com rather than copying them locally

Kustomize allows you to target local as well as remote resources too! Make use of the [Kyverno/policies](https://github.com/kyverno/policies) repository and use those instead of rewriting what's likely already been written. An example kustomize manifest that uses remote resources looks like this:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
commonLabels:
  app: kyverno
resources:
  # rules
  - https://raw.githubusercontent.com/kyverno/policies/952368d2083e511eb3128ee7f62aaf7ca36187f7/pod-security/baseline/disallow-capabilities/disallow-capabilities.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/disallow-host-namespaces/disallow-host-namespaces.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/disallow-host-path/disallow-host-path.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/disallow-host-ports/disallow-host-ports.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/disallow-privileged-containers/disallow-privileged-containers.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/disallow-proc-mount/disallow-proc-mount.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/disallow-selinux/disallow-selinux.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/restrict-apparmor-profiles/restrict-apparmor-profiles.yaml
  - https://raw.githubusercontent.com/kyverno/policies/03460e044fa8aeff9e055ecae176518e7869ac5c/pod-security/baseline/restrict-sysctls/restrict-sysctls.yaml
  - https://raw.githubusercontent.com/kyverno/policies/097ca254c5d52d05bf1aa385d140e8743b9f21ba/best-practices/disallow_default_namespace/disallow_default_namespace.yaml
```

### When referencing remote policies, target a commit hash or branch version rather than the main branch's head

In the previous example, you can see that explicit commit hashes are targeted rather than the latest and greatest. If you're using transformations, you'll likely have to stage the changes and update your patches on major updates.

### Avoid mutating resources with policies when possible

You can use an admission controller to [mutate offending resources](https://kyverno.io/docs/writing-policies/mutate/) rather than failing them outright. In my opinion this is something you should not do if you can avoid it. It is generally better to correct the resource manifest at its source rather than mutating it. Mutations are excellent for objects like logging sidecars and other container adjacent services.
