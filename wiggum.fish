function next
  ii todo ready --json --limit=1 | jq -r first.id
end

function fix --argument-names todo_id
    echo ">> fixing $todo_id"
    if ! ii job do $todo_id
      return 1
    end
    jj b m main -t @-
end

function fix-all
  while true
    set -l todo_id "$(next)"
    if test "null" = "$todo_id"
      echo "nothing left to do"
      break
    end
    fix $todo_id true || break
  end
end

