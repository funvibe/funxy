package analyzer

import (
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/symbols"
	"github.com/funvibe/funxy/internal/typesystem"
	"sort"
)

func (ctx *InferenceContext) SolveConstraints(table *symbols.SymbolTable) []error {
	var errors []error

	// Iteratively resolve constraints
	changed := true
	for changed {
		changed = false

		for _, c := range ctx.Constraints {
			if c.Kind == ConstraintUnify {
				left := c.Left.Apply(ctx.GlobalSubst)
				right := c.Right.Apply(ctx.GlobalSubst)
				subst, err := typesystem.UnifyAllowExtraWithResolver(left, right, table)
				if err == nil && len(subst) > 0 {
					prevLen := len(ctx.GlobalSubst)
					ctx.GlobalSubst = subst.Compose(ctx.GlobalSubst)
					if len(ctx.GlobalSubst) > prevLen {
						changed = true
					} else {
						// Simplification assume change if subst non-empty
						changed = true
					}
				}
			}
		}
	}

	// Final Pass
	for _, c := range ctx.Constraints {
		if c.Kind == ConstraintUnify {
			// Unification constraints are implicitly handled by the loop above.
			// Any remaining unification mismatch is implicitly an error that would have been caught
			// during the initial Unify call or will be caught if we re-check.
			// Ideally we should re-verify them here.
			left := c.Left.Apply(ctx.GlobalSubst)
			right := c.Right.Apply(ctx.GlobalSubst)
			_, err := typesystem.UnifyAllowExtraWithResolver(left, right, table)
			if err != nil {
				errors = append(errors, inferErrorf(c.Node, "type mismatch: %v", err))
			}

		} else if c.Kind == ConstraintImplements {
			if len(c.Args) == 0 {
				if c.Left != nil {
					c.Args = []typesystem.Type{c.Left}
				} else {
					continue
				}
			}

			// Resolve all arguments
			concreteArgs := make([]typesystem.Type, len(c.Args))
			hasAmbiguous := false
			isRigidError := false
			var rigidVars []string

			for i, arg := range c.Args {
				concrete := arg.Apply(ctx.GlobalSubst)
				// Manual lookup if Apply failed to resolve var
				if tv, ok := concrete.(typesystem.TVar); ok {
					if val, ok := ctx.GlobalSubst[tv.Name]; ok {
						concrete = val
					}
				}
				concreteArgs[i] = concrete

				// Check for Rigid TCon (lowercase name)
				isRigid := false
				if tCon, ok := concrete.(typesystem.TCon); ok && len(tCon.Name) > 0 && tCon.Name[0] >= 'a' && tCon.Name[0] <= 'z' {
					isRigid = true
				}

				if isVar(concrete) || isRigid {
					hasAmbiguous = true
					if isRigid {
						isRigidError = true
						if tCon, ok := concrete.(typesystem.TCon); ok {
							rigidVars = append(rigidVars, tCon.Name)
						}
					}
				}
			}

			if hasAmbiguous {
				// Check if the constraint is satisfied abstractly (by ActiveConstraints)
				if len(concreteArgs) == 1 {
					if typeHasConstraint(ctx, concreteArgs[0], c.Trait, table) {
						continue
					}
					// If we are here, it's ambiguous and not covered by constraints
					errors = append(errors, inferErrorf(c.Node, "ambiguous type %s: cannot determine instance for %s (add type annotation)", concreteArgs[0], c.Trait))
				} else {
					// MPTC ambiguous
					if ctx.HasMPTCConstraint(c.Trait, concreteArgs) {
						continue
					}

					// Functional Dependencies Logic
					if deps, ok := table.GetTraitFunctionalDependencies(c.Trait); ok && len(deps) > 0 {
						typeParamNames, _ := table.GetTraitTypeParams(c.Trait)
						paramIndices := make(map[string]int)
						for i, name := range typeParamNames {
							paramIndices[name] = i
						}

						for _, dep := range deps {
							// Check if all "From" arguments are resolved (not variables)
							canResolve := true
							for _, fromVar := range dep.From {
								idx, ok := paramIndices[fromVar]
								if !ok || idx >= len(concreteArgs) {
									canResolve = false
									break
								}
								// Apply current subst to ensure we see latest state
								arg := concreteArgs[idx].Apply(ctx.GlobalSubst)
								if isVar(arg) {
									canResolve = false
									break
								}
							}

							if canResolve {
								allImpls := table.GetAllImplementations()[c.Trait]
								for _, implDef := range allImpls {
									if len(implDef.TargetTypes) != len(concreteArgs) {
										continue
									}

									// Check unification of "From" args
									match := true
									subst := make(typesystem.Subst)

									for _, fromVar := range dep.From {
										idx := paramIndices[fromVar]

										// Rename instance vars unique to this attempt
										instArg := symbols.RenameTypeVars(implDef.TargetTypes[idx], "inst")

										// Unify concrete (known) with instance (generic)
										s, err := typesystem.Unify(concreteArgs[idx], instArg)
										if err != nil {
											match = false
											break
										}
										subst = subst.Compose(s)
									}

									if match {
										// Found a match based on functional dependency inputs.
										// Enforce outputs.
										for _, toVar := range dep.To {
											idx := paramIndices[toVar]

											instArg := symbols.RenameTypeVars(implDef.TargetTypes[idx], "inst")
											expectedType := instArg.Apply(subst)

											// Unify the inferred type with the constraint argument
											// This fixes the type variable in the constraint!
											s, err := typesystem.Unify(concreteArgs[idx], expectedType)
											if err == nil && len(s) > 0 {
												ctx.GlobalSubst = s.Compose(ctx.GlobalSubst)
												changed = true

												// Update local args for immediate consistency in this loop
												concreteArgs[idx] = concreteArgs[idx].Apply(s)
												c.Args[idx] = c.Args[idx].Apply(s)
											}
										}
										// We found the determining instance. Stop searching.
										// (Consistency check ensures uniqueness)
										break
									}
								}
							}
						}

						// If we resolved something using FunDeps, we continue.
						// Should we skip the heuristic? Yes, FunDeps are stricter.
						if changed {
							continue
						}
					}

					// Attempt to find instance even with unresolved variables if it's uniquely determined
					// (Functional Dependency heuristic: if partial match is unique, assume it)
					// Filter concrete args: keep resolved ones
					// This is risky if multiple instances could match the resolved parts.
					// But for now, if we have [Option<Int>, ?], and only one instance [Option<Int>, Int],
					// we can optimistically assume it matches.

					// Count resolved args
					resolvedCount := 0
					for _, arg := range concreteArgs {
						if !isVar(arg) {
							resolvedCount++
						}
					}

					// If we have at least one resolved arg, try to find unique partial match
					if resolvedCount > 0 {
						matchingInstances := table.FindMatchingInstances(c.Trait, concreteArgs, isVar)
						if len(matchingInstances) == 1 {
							// Unique match found!
							// We implicitly assume the unresolved vars will resolve to the instance's types.
							// To enforce this, we should probably add Unify constraints?
							// But SolveConstraints is iterating constraints.
							// For now, let's just accept it as valid if unique.
							// The actual type unification should happen elsewhere or later.
							// Wait, if we don't unify, 'e' remains unknown.
							// If we accept it here, we suppress the error.
							// But 'e' is used in the function call return type.
							// If we don't fix 'e', subsequent inference might fail or remain polymorphic.

							// If we find a unique instance, we can extract the concrete types for the variables
							// and unify them!
							instDef := matchingInstances[0]

							// Need to map instance definition types to concreteArgs
							// Instance: Collection<Option<Int>, Int>
							// ConcreteArgs: [Option<Int>, t1]

							// Re-construct instance types
							instTypes := instDef.TargetTypes

							// Unify resolved vars with instance types
							for i, arg := range concreteArgs {
								if isVar(arg) {
									// This argument is unresolved (t1).
									// The corresponding instance type (Int) should be used.
									// But instance type might be generic (List<a>).
									// We need to rename instance vars to avoid collision.
									// Rename vars manually here since helper is not available/exported easily
									instType := instTypes[i]
									tVars := instType.FreeTypeVariables()
									tSubst := make(typesystem.Subst)
									for _, v := range tVars {
										tSubst[v.Name] = typesystem.TVar{Name: v.Name + "_inst"}
									}
									instType = instType.Apply(tSubst)

									// We can force unification here!
									// This effectively "resolves" the ambiguous variable.
									// However, we are in SolveConstraints, modifying subst?
									// Yes, we can try to Unify and update GlobalSubst?

									subst, err := typesystem.Unify(arg, instType)
									if err == nil && len(subst) > 0 {
										ctx.GlobalSubst = subst.Compose(ctx.GlobalSubst)
										changed = true // Trigger re-iteration

										// Manually apply to concreteArgs so we can verify match NOW
										concreteArgs[i] = concreteArgs[i].Apply(subst)

										// ALSO apply to the constraint args itself!
										c.Args[i] = c.Args[i].Apply(subst)
									}
								}
							}

							// Continue to next iteration to let changes propagate
							// continue
							// Instead of continuing (which moves to next constraint in Final Pass),
							// we should restart or assume it's fixed?
							// SolveConstraints has outer loop 'changed'.
							// If we set changed=true, we should probably break inner loop or just let it finish.

							// If we found unique match, we consider this constraint potentially satisfied.
							// But we need to ensure the variable 'arg' (t1) is actually updated in GlobalSubst.
							// If it is, then next pass will see concrete types.

							// CRITICAL: We also need to add the matched instance to "InferredConstraints" or somehow
							// allow SolveWitness to find it later if variables are still present?
							// Or better, SolveWitness should use the same logic!
							// SolveWitness currently fails if args have vars (unless Rigid).
							// We need to update SolveWitness to also support unique partial match.

							continue
						}
					}

					if isRigidError {
						// Infer constraint for rigid variables!
						// 1. Add to InferredConstraints
						varName := ""
						if tv, ok := concreteArgs[0].(typesystem.TVar); ok {
							varName = tv.Name
						} else if tc, ok := concreteArgs[0].(typesystem.TCon); ok {
							varName = tc.Name
						}

						exists := false
						for _, ic := range ctx.InferredConstraints {
							if ic.Trait == c.Trait && len(ic.Args) == len(concreteArgs) {
								match := true
								for i, a := range ic.Args {
									if a.String() != concreteArgs[i].String() {
										match = false
										break
									}
								}
								if match {
									exists = true
									break
								}
							}
						}

						if !exists && varName != "" {
							ctx.InferredConstraints = append(ctx.InferredConstraints, Constraint{
								Kind:  ConstraintImplements,
								Trait: c.Trait,
								Args:  concreteArgs,
								Node:  c.Node,
							})
						}

						// Do not report error, assume inferred
						continue
					} else {
						errors = append(errors, inferErrorf(c.Node, "ambiguous types for trait %s (unresolved inference variables)", c.Trait))
					}
				}
				continue
			}

			// Check implementation for concrete types
			unwrappedArgs := make([]typesystem.Type, len(concreteArgs))
			for i, arg := range concreteArgs {
				checkType := typesystem.UnwrapUnderlying(arg)
				if checkType == nil {
					checkType = arg
				}
				unwrappedArgs[i] = checkType
			}

			if !table.IsImplementationExists(c.Trait, unwrappedArgs) {
				// Check if satisfied by generic constraints (simple check for now)
				if len(unwrappedArgs) > 0 && typeHasConstraint(ctx, concreteArgs[0], c.Trait, table) {
					continue
				}
				// Check strict MPTC constraint if available
				if ctx.HasMPTCConstraint(c.Trait, unwrappedArgs) {
					continue
				}

				// Format error message based on number of arguments
				if len(unwrappedArgs) == 1 {
					errors = append(errors, inferErrorf(c.Node, "type %s does not implement trait %s", unwrappedArgs[0], c.Trait))
				} else {
					errors = append(errors, inferErrorf(c.Node, "types %v do not implement trait %s", unwrappedArgs, c.Trait))
				}
			}
		}
	}

	return errors
}

// getKeys returns sorted keys of a substitution map
func getKeys(s typesystem.Subst) []string {
	keys := []string{}
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isVar(t typesystem.Type) bool {
	if _, ok := t.(typesystem.TVar); ok {
		return true
	}
	if tCon, ok := t.(typesystem.TCon); ok {
		// Treat lowercase TCons as rigid type variables
		if len(tCon.Name) > 0 && tCon.Name[0] >= 'a' && tCon.Name[0] <= 'z' {
			return true
		}
	}
	return false
}

// SolveWitness resolves a witness (dictionary) for a given trait and type arguments.
// It returns an AST expression (Identifier or CallExpression) representing the dictionary.
// Used for dictionary passing transformation.
func (ctx *InferenceContext) SolveWitness(node ast.Node, traitName string, args []typesystem.Type, table *symbols.SymbolTable) (ast.Expression, error) {
	// 1. Resolve arguments
	resolvedArgs := make([]typesystem.Type, len(args))
	hasVar := false
	for i, arg := range args {
		resolved := arg.Apply(ctx.GlobalSubst)
		resolvedArgs[i] = resolved
		if isVar(resolved) {
			hasVar = true
		}
	}

	// 2. Case A (Local): Generic Type Variable (T: Show)
	if hasVar {
		// For MPTC with vars, we also expect a named parameter if it was in the function signature.
		if len(resolvedArgs) > 0 {
			var varName string
			if tv, ok := resolvedArgs[0].(typesystem.TVar); ok {
				varName = tv.Name
			} else if tc, ok := resolvedArgs[0].(typesystem.TCon); ok {
				varName = tc.Name
			}

			if varName != "" {
				// Strategy 1: Construct MPTC witness name: $dict_Type_Trait_Arg1_Arg2...
				paramName := GetWitnessParamName(varName, traitName)
				if len(resolvedArgs) > 1 {
					for _, arg := range resolvedArgs[1:] {
						paramName += "_" + arg.String()
					}
				}

				if table.IsDefined(paramName) {
					return &ast.Identifier{Value: paramName}, nil
				}

				// Strategy 2: Check for SuperTraits (e.g. if we have Monad<m>, we have Applicative<m>)
				// Iterate over active constraints to find one that implies the required trait
				if constraints, ok := ctx.ActiveConstraints[varName]; ok {
					for _, c := range constraints {
						if c.Trait != traitName && isTraitSubclass(c.Trait, traitName, table) {
							// Found constraint that implies required trait
							cFullArgs := make([]typesystem.Type, 0, 1+len(c.Args))
							cFullArgs = append(cFullArgs, typesystem.TVar{Name: varName})
							cFullArgs = append(cFullArgs, c.Args...)

							witnessExpr, err := ctx.SolveWitness(node, c.Trait, cFullArgs, table)
							if err == nil {
								path := findSuperTraitPath(c.Trait, traitName, table)
								if path != nil {
									var expr ast.Expression = witnessExpr
									for _, index := range path {
										expr = &ast.IndexExpression{
											Left: &ast.MemberExpression{
												Left:   expr,
												Member: &ast.Identifier{Value: "Supers"},
											},
											Index: &ast.IntegerLiteral{Value: int64(index)},
										}
									}
									return expr, nil
								}
							}
						}
					}
				}
			}
		}

		// Check if the ambiguity is due to Rigid Type Variables (Generics)
		isAllRigid := true
		for _, arg := range resolvedArgs {
			if !isVar(arg) {
				isAllRigid = false
				break
			}
		}

		// Attempt unique partial match logic in SolveWitness too!
		// If we have mixed rigid/concrete/unresolved, and unique instance exists.
		matchingInstances := table.FindMatchingInstances(traitName, resolvedArgs, isVar)
		if len(matchingInstances) == 1 {
			// Found unique match!
			// We can proceed as if we found it.
			// Need to instantiate the instance similarly to Case B.

			implDef := matchingInstances[0]
			evidenceName := implDef.ConstructorName
			// Proceed to instantiate...
			// We need to establish substitution from instance types to resolvedArgs.
			// But resolvedArgs might have vars.
			// Unify instance types with resolvedArgs to get 'subst'.

			// subst, _ := table.FindMatchingImplementation(traitName, resolvedArgs)
			// Wait, FindMatchingImplementation requires full match?
			// Let's manually build subst.

			subst := make(typesystem.Subst)
			renamedTargetTypes := make([]typesystem.Type, len(implDef.TargetTypes))
			for j, t := range implDef.TargetTypes {
				// Manually rename vars
				tVars := t.FreeTypeVariables()
				tSubst := make(typesystem.Subst)
				for _, v := range tVars {
					tSubst[v.Name] = typesystem.TVar{Name: v.Name + "_inst"}
				}
				renamedTargetTypes[j] = t.Apply(tSubst)
			}

			for j, implArg := range renamedTargetTypes {
				s, err := typesystem.Unify(implArg, resolvedArgs[j])
				if err == nil {
					subst = s.Compose(subst)
				}
			}

			// Continue to logic below using 'implDef', 'subst', 'evidenceName'
			// Copy-paste logic from Case B (Constructor / Generic Instance)

			ctorArgs := []ast.Expression{}
			for _, req := range implDef.Requirements {
				var reqType typesystem.Type = typesystem.TVar{Name: req.TypeVar}
				reqType = reqType.Apply(subst)
				reqArgs := make([]typesystem.Type, len(req.Args))
				for i, a := range req.Args {
					reqArgs[i] = a.Apply(subst)
				}
				fullReqArgs := []typesystem.Type{reqType}
				fullReqArgs = append(fullReqArgs, reqArgs...)

				argWitness, err := ctx.SolveWitness(node, req.Trait, fullReqArgs, table)
				if err != nil {
					return nil, err
				}
				ctorArgs = append(ctorArgs, argWitness)
			}

			// Check symbol kind to decide CallExpression vs Identifier
			sym, foundSym := table.Find(evidenceName)
			isFunc := false
			if foundSym {
				t := typesystem.UnwrapUnderlying(sym.Type)
				if t == nil {
					t = sym.Type
				}
				if _, ok := t.(typesystem.TFunc); ok {
					isFunc = true
				}
				if _, ok := t.(typesystem.TForall); ok {
					isFunc = true
				}
			}

			if isFunc {
				return &ast.CallExpression{
					Function:  &ast.Identifier{Value: evidenceName},
					Arguments: ctorArgs,
				}, nil
			} else {
				return &ast.Identifier{Value: evidenceName}, nil
			}
		}

		if isAllRigid {
			// Infer constraint for rigid variables!
			// This allows generic functions and instances to implicitly depend on constraints derived from usage.

			// 1. Generate witness parameter name
			varName := ""
			if tv, ok := resolvedArgs[0].(typesystem.TVar); ok {
				varName = tv.Name
			} else if tc, ok := resolvedArgs[0].(typesystem.TCon); ok {
				varName = tc.Name
			}

			paramName := GetWitnessParamName(varName, traitName)
			if len(resolvedArgs) > 1 {
				for _, arg := range resolvedArgs[1:] {
					paramName += "_" + arg.String()
				}
			}

			// 2. Add to InferredConstraints
			// Check if already exists to avoid duplicates
			exists := false
			for _, c := range ctx.InferredConstraints {
				if c.Trait == traitName && len(c.Args) == len(resolvedArgs) {
					match := true
					for i, a := range c.Args {
						// Simple string comparison for rigid vars
						if a.String() != resolvedArgs[i].String() {
							match = false
							break
						}
					}
					if match {
						exists = true
						break
					}
				}
			}

			if !exists {
				ctx.InferredConstraints = append(ctx.InferredConstraints, Constraint{
					Kind:  ConstraintImplements,
					Trait: traitName,
					Args:  resolvedArgs,
					Node:  node,
				})
			}

			// 3. Return witness Identifier
			return &ast.Identifier{Value: paramName}, nil
		}

		return nil, inferErrorf(node, "witness resolution failed for %s with args %v", traitName, resolvedArgs)
	}

	// 3. Case B (Global): Concrete Instance (Int: Show)
	evidenceName := ""

	// Construct Type Key for lookup
	key := GetEvidenceKey(traitName, resolvedArgs)

	// Look up in Evidence Table
	if name, ok := table.GetEvidence(key); ok {
		evidenceName = name
	}

	ok := (evidenceName != "")
	if ok {
		// Check if it's a constructor function (starts with $ctor)
		// Or if it's a constant.
		// Check symbol type in table.
		sym, found := table.Find(evidenceName)
		isConstructor := false
		if found {
			t := typesystem.UnwrapUnderlying(sym.Type)
			if t == nil {
				t = sym.Type
			}
			if _, isFunc := t.(typesystem.TFunc); isFunc {
				isConstructor = true
			} else if _, isPoly := t.(typesystem.TForall); isPoly {
				isConstructor = true
			}
		}

		if isConstructor {
			// It is a constructor (Generic Instance)
			// We need to call it with witnesses for its requirements.

			// Use FindInstance to get the instance definition and substitution
			// We already found the evidence name, so we just need the definition that generated it.
			// Re-finding via FindInstance is safer to get the Subst.
			implDef, subst, foundInst := table.FindInstance(traitName, resolvedArgs)

			if foundInst && implDef.ConstructorName == evidenceName {
				ctorArgs := []ast.Expression{}
				for _, req := range implDef.Requirements {
					// req is Constraint {TypeVar, Trait, Args}
					// Apply substitution to req types

					// Apply to TypeVar
					var reqType typesystem.Type = typesystem.TVar{Name: req.TypeVar}
					// If TypeVar is not in subst (e.g. it was "Int" in instance def), it remains as is?
					// No, if instance was `instance Show Int`, req might be empty.
					// If `instance Show [a]`, req is `Show a`. TypeVar is `a`.
					// `a` should be in subst.
					reqType = reqType.Apply(subst)

					// Apply to Args
					reqArgs := make([]typesystem.Type, len(req.Args))
					for i, a := range req.Args {
						reqArgs[i] = a.Apply(subst)
					}

					// Construct full args for SolveWitness
					fullReqArgs := []typesystem.Type{reqType}
					fullReqArgs = append(fullReqArgs, reqArgs...)

					argWitness, err := ctx.SolveWitness(node, req.Trait, fullReqArgs, table)
					if err != nil {
						return nil, err
					}
					ctorArgs = append(ctorArgs, argWitness)
				}

				return &ast.CallExpression{
					Function:  &ast.Identifier{Value: evidenceName},
					Arguments: ctorArgs,
				}, nil
			}

			// Fallback if FindInstance failed to match (shouldn't happen if GetEvidence succeeded)
			// But maybe GetEvidence was exact match on Key?
		} else {
			// Case B: Global Constant (e.g. $impl_Show_Int)
			return &ast.Identifier{Value: evidenceName}, nil
		}
	}

	// Fallback for Generic Instances that are NOT in EvidenceTable (because Key didn't match exactly)
	// Try FindInstance
	if implDef, subst, found := table.FindInstance(traitName, resolvedArgs); found {
		evidenceName = implDef.ConstructorName

		// Logic similar to above for Constructor
		ctorArgs := []ast.Expression{}
		for _, req := range implDef.Requirements {
			var reqType typesystem.Type = typesystem.TVar{Name: req.TypeVar}
			reqType = reqType.Apply(subst)

			reqArgs := make([]typesystem.Type, len(req.Args))
			for i, a := range req.Args {
				reqArgs[i] = a.Apply(subst)
			}

			fullReqArgs := []typesystem.Type{reqType}
			fullReqArgs = append(fullReqArgs, reqArgs...)

			argWitness, err := ctx.SolveWitness(node, req.Trait, fullReqArgs, table)
			if err != nil {
				return nil, err
			}
			ctorArgs = append(ctorArgs, argWitness)
		}

		// If arguments are empty, it might be a constant or a 0-arg function?
		// Instance constructors are functions even if 0 args?
		// Check symbol kind
		sym, foundSym := table.Find(evidenceName)
		isFunc := false
		if foundSym {
			t := typesystem.UnwrapUnderlying(sym.Type)
			if t == nil {
				t = sym.Type
			}
			if _, ok := t.(typesystem.TFunc); ok {
				isFunc = true
			}
			if _, ok := t.(typesystem.TForall); ok {
				isFunc = true
			}
		}

		if isFunc {
			return &ast.CallExpression{
				Function:  &ast.Identifier{Value: evidenceName},
				Arguments: ctorArgs,
			}, nil
		} else {
			return &ast.Identifier{Value: evidenceName}, nil
		}
	}

	// Fallback for built-in instances that might not be in EvidenceTable yet
	if !ok {
		// Fallback: Check ActiveConstraints for Rigid Type Variables (TCons)
		// This handles cases inside instance definitions where 'a' is a rigid TCon, not a TVar.
		if len(resolvedArgs) > 0 {
			if ctx.HasMPTCConstraint(traitName, resolvedArgs) {
				var name string
				arg0 := typesystem.UnwrapUnderlying(resolvedArgs[0])
				if arg0 == nil {
					arg0 = resolvedArgs[0]
				}

				if tCon, ok := arg0.(typesystem.TCon); ok {
					name = tCon.Name
				} else if tVar, ok := arg0.(typesystem.TVar); ok {
					name = tVar.Name
				}

				if name != "" {
					return &ast.Identifier{Value: GetWitnessParamName(name, traitName)}, nil
				}
			}
		}
	}

	return nil, inferErrorf(node, "evidence not found for %s", key)
}

// findSuperTraitPath returns a list of indices to traverse from subTrait to superTrait
func findSuperTraitPath(sub, super string, table *symbols.SymbolTable) []int {
	if sub == super {
		return []int{}
	}

	// BFS to find shortest path
	type step struct {
		trait string
		path  []int
	}
	queue := []step{{trait: sub, path: []int{}}}
	visited := make(map[string]bool)
	visited[sub] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		supers, _ := table.GetTraitSuperTraits(curr.trait)
		for i, s := range supers {
			if s == super {
				return append(curr.path, i)
			}
			if !visited[s] {
				visited[s] = true
				newPath := make([]int, len(curr.path))
				copy(newPath, curr.path)
				newPath = append(newPath, i)
				queue = append(queue, step{trait: s, path: newPath})
			}
		}
	}
	return nil
}

// Helper to match types for substitution extraction (simple structural matching)
func matchType(pattern, target typesystem.Type, subst typesystem.Subst) {
	switch p := pattern.(type) {
	case typesystem.TVar:
		subst[p.Name] = target
	case typesystem.TApp:
		if tApp, ok := target.(typesystem.TApp); ok {
			matchType(p.Constructor, tApp.Constructor, subst)
			if len(p.Args) == len(tApp.Args) {
				for i := range p.Args {
					matchType(p.Args[i], tApp.Args[i], subst)
				}
			}
		}
	case typesystem.TCon:
		// Rigid match, nothing to capture
	}
}
