function next
  ii todo ready --json --limit=1 | jq -r first.id
end

function fix --argument-names todo_id move_main
    echo ">> fixing $todo_id"
    jj new main
    echo ">> made empty change"
    set -l change_id (jj show @ -T change_id)
    echo ">> change_id=$change_id"
    set -l workspace_dir (ii session start "$todo_id" --rev=$change_id) || return 1
    echo ">> workspace_dir=$workspace_dir"
    pushd $workspace_dir
    set -l prompt "complete this task:"\n\n"$(ii todo show $todo_id)"
    echo ">> prompt:"
    echo "$prompt"
    echo "<< prompt"
    echo "$prompt" | opencode run
    echo ">> opencode done"

    if ! go test ./...
      ii session fail $todo_id
      echo "tests failed; rejecting change"
      popd
      return 1
    end
    echo ">> tests passed"

    set -l todo_json "$(ii todo show --json $todo_id)"
    set -l todo_title "$(echo "$todo_json" | jq -r first.title)"
    set -l todo_description "$(echo "$todo_json" | jq -r first.description)"
    echo ">> todo_title=$todo_title"
    echo ">> committing"
    jj commit --message="$todo_title"\n\n"$todo_description"
    echo ">> committed"
    if test move_main = true
      jj bookmark move main --to @-
      echo ">> advanced main bookmark"
    end

    ii session done $todo_id
    echo ">> closed session"
    popd
end

function wiggum
  while true
    set -l todo_id "$(next)"
    if test "null" = "$todo_id"
      echo "nothing left to do"
      break
    end
    fix $todo_id true || break
  end
end
