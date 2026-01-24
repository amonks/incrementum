function next
  ii todo ready --json --limit=1 | jq -r first.id
end

function fix --argument-names todo_id --argument-names move_main
    echo ">> fixing $todo_id"
    ii todo start "$todo_id"
    jj new main
    echo ">> made empty change"
    set -l change_id (jj show @ -T change_id --no-patch)
    echo ">> change_id=$change_id"
    set -l prompt "complete this task:"\n\n"$(ii todo show $todo_id)"
    echo ">> prompt:"
    echo "$prompt"
    echo "<< prompt"
    echo "$prompt" | opencode run
    echo ">> opencode done"

    if ! go test ./...
      ii todo reopen "$todo_id"
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
    if test "$move_main" = true
      jj bookmark move main --to @-
      echo ">> advanced main bookmark"
    end

    ii todo finish "$todo_id"
    echo ">> closed todo"
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

function improvement-loop
  while true
    echo ">> running an improvement"

    # run the improvement
    cat prompt.md | opencode run
    if ! go test ./...
      # the tests failed. improvement rejected. exit.
      echo ">> test failure"
      return 1
    end

    echo ">> tests passed; improvement is valid; continue"
  end
end

function wiggum-todo --argument-names todo_id
  set -l last_commit_id "$(jj show -T commit_id --no-patch)"
  set -l consecutive_failures 0

  while true
    echo ">> executing todo $todo_id"
    echo ">> last_commit_id=$last_commit_id"

    # run opencode against the todo
    set -l prompt "Figure out what the highest priority task is towards implementing the todo below, and complete it. Do tdd (write a failing test, watch it fail, make it pass). Make sure 'go test ./...' passes. If there's nothing left to do, that's ok: exit without making changes.

$(ii todo show $todo_id)"
    echo "$prompt" | opencode run

    # test the result
    if ! go test ./...
      # if the tests failed, increment consecutive_failures, restore, and
      # try again
      echo ">> test failure"
      set -l consecutive_failures (math $consecutive_failures + 1)
      if test $consecutive_failures -ge 3
        echo ">> too many failures; exiting"
        return 1
      end
      jj restore --from "$last_commit_id"
      echo ">> restored to $last_commit_id"
      continue
    end
    set -l consecutive_failures 0

    # the tests succeeded; check if we made any changes
    jj debug snapshot
    set -l new_commit_id "$(jj show -T commit_id --no-patch)"
    echo ">> captured post-work snapshot. new_commit_id=$new_commit_id"

    # if we made no changes, the project is done. exit.
    if test "$last_commit_id" = "$new_commit_id"
      echo ">> no changes; done"
      break
    end

    # if we made changes, there may still be more work to do. update
    # last_commit_id and continue the loop.
    echo ">> made changes: last_commit_id=$last_commit_id, new_commit_id=$new_commit_id"
    set last_commit_id "$new_commit_id"
    echo ">> updated last_commit_id for next turn. last_commit_id=$last_commit_id"
    echo ">> loop again..."
  end
end

function wiggum --argument-names spec
  set -l last_commit_id "$(jj show -T commit_id --no-patch)"
  set -l consecutive_failures 0

  while true
    echo ">> executing against $spec"
    echo ">> last_commit_id=$last_commit_id"

    # run opencode against the spec
    set -l prompt "Figure out what the highest priority task is towards implementing the $spec spec, and complete it. Do tdd (write a failing test, watch it fail, make it pass). If there's nothing left to do, that's ok: exit without making changes."
    echo "$prompt" | opencode run

    # test the result
    if ! go test ./...
      # if the tests failed, increment consecutive_failures, restore, and
      # try again
      echo ">> test failure"
      set -l consecutive_failures (math $consecutive_failures + 1)
      if test $consecutive_failures -ge 3
        echo ">> too many failures; exiting"
        return 1
      end
      jj restore --from "$last_commit_id"
      echo ">> restored to $last_commit_id"
      continue
    end
    set -l consecutive_failures 0

    # the tests succeeded; check if we made any changes
    jj debug snapshot
    set -l new_commit_id "$(jj show -T commit_id --no-patch)"
    echo ">> captured post-work snapshot. new_commit_id=$new_commit_id"

    # if we made no changes, the project is done. exit.
    if test "$last_commit_id" = "$new_commit_id"
      echo ">> no changes; done"
      break
    end

    # if we made changes, there may still be more work to do. update
    # last_commit_id and continue the loop.
    echo ">> made changes: last_commit_id=$last_commit_id, new_commit_id=$new_commit_id"
    set last_commit_id "$new_commit_id"
    echo ">> updated last_commit_id for next turn. last_commit_id=$last_commit_id"
    echo ">> loop again..."
  end
end
