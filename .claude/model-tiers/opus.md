You've likely already inferred the conventions in the base tier from
reading the code — treat them as context, not a checklist to re-verify
line by line. Where you'd otherwise stop and ask before a structural
change, go ahead, as long as the documented contracts still hold
(directory-as-config, longest-substring-wins, base-applies-regardless-
of-model, global-before-project ordering) and the test suite still
encodes them.
