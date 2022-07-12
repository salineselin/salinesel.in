---
title: "Good Kyverno Admission Control Patterns"
description:
date: 2022-07-11T16:12:18-06:00
image: "pexels-photo-1169754.jpeg"
slug: kyverno-patterns
draft: true
tags:
  - kubernetes
---

Kyverno is an admission controller used to add policies to your cluster. The basic principle of an admission controller is to intercept incoming requests to a given kubernetes apiserver and check if a field matches an expression, then approve or deny the request based on that determination. If you're not familiar with the concepts surrounding admission controllers already, I'd recommend reading [the official documentation](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#what-are-they) on admission controllers.

If you're in the process of deciding what admisson controller to use in your environment, consider the background of the team that will be maintaining the policies. Do they all know how to program in go? If so, [OPA](https://www.openpolicyagent.org/) may be a better choice for you. I personally believe that Kyverno is a better implementation because the barrier for reading and writing policies is significantly lower than OPA. I have yet to run into a scenario where I need more verbose or complex syntax beyond what ships in vanilla kyverno

## the gist

- have some way to whitelist resources
- narrow down the scope of your whitelists as much as possible
- have some way to unit test against admission controller policies when working with k8s manifests in a github repository
- separate your kyverno installation from the underlying policies
- have some way to toggle rule actions between audit and enforcement
- add remote policies using raw.githubusercontent.com rather than copying them locally
- when referencing remote policies, target a commit hash rather than the main branch's head
- avoid mutating resources with policies when possible

## recommendations

### have some way to whitelist resources

It is inevitable that you will have resources in your kubernetes clusters that violate policies.

### narrow down the scope of your whitelists as much as possible

### have some way to unit test against admission controller policies when working with k8s manifests in a github repository

### separate your kyverno installation from the underlying policies

### have some way to toggle rule actions between audit and enforcement

### add remote policies using raw.githubusercontent.com rather than copying them locally

### when referencing remote policies, target a commit hash rather than the main branch's head

### avoid mutating resources with policies when possible

# frequently asked questions

## why not install with helm?

Helm is a perfectly fine tool to use for default installations, but when you're making any complex mutations to the underlying resources in the chart, you become constrained by the solutions the helm chart maintainers have already thought of.

## where can I get some policies from?
