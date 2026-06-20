# Test incident: a diagnostic finding was acted on as authority

Status: resolved 2026-06-20

## Root cause

A doctor finding was treated as authority and a destructive cleanup ran from
filesystem evidence alone.

## Fix

Build an authority chain before destructive action.
