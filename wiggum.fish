function next
  incr todo ready --json --limit=1 | jq -r first.id
end

function fix --argument-names todo_id
    jj new main
    set -l change_id (jj show @ -T change_id)
    echo "starting session for $todo_id"
    cd (incr session start "$todo_id" --rev=$change_id)
    set -l prompt "complete this task:"\n\n"$(incr todo show $todo_id)"
    echo "$prompt"
    echo "$prompt" | opencode run

    if ! go test ./...
      # incr session fail $todo_id
      echo "tests failed; rejecting change"
      return
    end

    echo "success; committing"
    set -l todo_json "$(incr todo show --json $todo_id)"
    set -l todo_title "$(echo "$todo_json" | jq -r first.title)"
    set -l todo_description "$(echo "$todo_json" | jq -r first.description)"
    jj commit --message="$todo_title"\n\n"$todo_description"
    jj bookmark move main --to @-

    echo "done"
    # incr session done $todo_id
end

function wiggum
  while true
    set -l todo_id "$(next)"
    if test "null" = "$todo_id"
      echo "nothing left to do"
      break
    end
    fix $todo_id || break
  end
end

