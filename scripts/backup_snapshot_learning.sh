#!/usr/bin/env bash
# This script is a learning exercise for Linux backups and snapshots.
# It is intentionally written with many comments so you can read it like notes.
# It does not run by itself just because the file exists.
# Keep DRY_RUN="true" until you understand and edit the settings below.

set -Eeuo pipefail
# set makes Bash stricter so errors are easier to notice.
# -E keeps ERR traps working inside functions.
# -e stops the script when a command fails.
# -u stops the script when a variable was never set.
# -o pipefail makes a pipeline fail if any command inside it fails.

DRY_RUN="true"
# DRY_RUN="true" means the script prints commands instead of running them.
# Change this to "false" only after you understand where your backups go.

ENABLE_TIMESHIFT="true"
# Timeshift is for system snapshots, similar to restore points.
# It helps when an update or config change breaks Linux.
# Timeshift is not enough by itself if your whole computer or drive dies.

ENABLE_RESTIC="true"
# restic is for real versioned backups of your files.
# Versioned means you can restore older copies if a file becomes corrupted.

ENABLE_BORG="false"
# BorgBackup is another serious versioned backup tool.
# This script includes it as an example, but keeps it off by default.
# Usually choose either restic or Borg first, not both at the same time.

ENABLE_RSYNC_MIRROR="false"
# rsync can make a simple mirror copy of files.
# A mirror is useful, but it can copy corruption or deletions too.
# For corrupted-file protection, restic or Borg is usually safer.

BACKUP_SOURCE_HOME="$HOME"
# This is the main folder to back up.
# $HOME usually means /home/your-username.

RESTIC_REPOSITORY="/mnt/backup-drive/restic-repo"
# This is where restic stores backups.
# Change this to an external drive path or a cloud-mounted path.
# Do not rely only on a folder inside the same computer.

BORG_REPOSITORY="/mnt/backup-drive/borg-repo"
# This is where Borg stores backups.
# Change this to an external drive path if you decide to use Borg.

RSYNC_DESTINATION="/mnt/backup-drive/home-mirror"
# This is where rsync would mirror your home folder.
# A mirror is not the same thing as a versioned backup.

RESTIC_KEEP_HOURLY="24"
# This keeps up to 24 hourly restic backups.

RESTIC_KEEP_DAILY="30"
# This keeps up to 30 daily restic backups.

RESTIC_KEEP_WEEKLY="8"
# This keeps up to 8 weekly restic backups.

BORG_KEEP_HOURLY="24"
# This keeps up to 24 hourly Borg backups.

BORG_KEEP_DAILY="30"
# This keeps up to 30 daily Borg backups.

BORG_KEEP_WEEKLY="8"
# This keeps up to 8 weekly Borg backups.

EXCLUDES=(
  "$HOME/.cache"
  "$HOME/.local/share/Trash"
  "$HOME/Downloads"
)
# EXCLUDES lists folders you probably do not need to back up.
# You can remove "$HOME/Downloads" if your downloads matter to you.

print_line() {
  # This function prints a simple divider.
  printf '%s\n' "------------------------------------------------------------"
}

explain_plan() {
  # This function prints the learning exercise and the plan.
  print_line
  # This prints the top divider.
  printf '%s\n' "Learning exercise: backup vs snapshot"
  # This prints the lesson title.
  print_line
  # This prints another divider.
  printf '%s\n' "1. Snapshot: Timeshift saves system states for rollback."
  # This explains snapshots.
  printf '%s\n' "2. Backup: restic or Borg saves file history to another place."
  # This explains backups.
  printf '%s\n' "3. Mirror: rsync copies the latest files, but does not keep history."
  # This explains mirrors.
  printf '%s\n' "4. Best protection: keep versioned backups outside this computer."
  # This explains the important safety rule.
  printf '%s\n' "5. First run should be DRY_RUN=true so you can inspect commands."
  # This reminds you to avoid accidental changes.
  print_line
  # This prints the bottom divider.
}

run_cmd() {
  # This function either prints a command or runs it.
  if [[ "$DRY_RUN" == "true" ]]; then
    # This branch is used when DRY_RUN is turned on.
    printf 'DRY RUN: %q ' "$@"
    # This prints the command safely quoted.
    printf '\n'
    # This finishes the dry-run line.
  else
    # This branch is used when DRY_RUN is turned off.
    "$@"
    # This actually runs the command that was passed into the function.
  fi
}

need_command() {
  # This function checks whether a tool exists.
  local command_name="$1"
  # This stores the command name given to the function.
  if command -v "$command_name" >/dev/null 2>&1; then
    # This checks if Linux can find the command.
    printf '%s\n' "OK: found $command_name"
    # This reports that the command exists.
  else
    # This branch runs when the command is missing.
    printf '%s\n' "MISSING: $command_name"
    # This reports that the command is missing.
    printf '%s\n' "Install idea on Ubuntu: sudo apt install $command_name"
    # This gives the basic Ubuntu install command.
  fi
}

check_tools() {
  # This function checks the tools used by the enabled sections.
  print_line
  # This prints a divider.
  printf '%s\n' "Checking tools"
  # This prints the section title.
  [[ "$ENABLE_TIMESHIFT" == "true" ]] && need_command timeshift
  # This checks Timeshift only when its section is enabled.
  [[ "$ENABLE_RESTIC" == "true" ]] && need_command restic
  # This checks restic only when its section is enabled.
  [[ "$ENABLE_BORG" == "true" ]] && need_command borg
  # This checks Borg only when its section is enabled.
  [[ "$ENABLE_RSYNC_MIRROR" == "true" ]] && need_command rsync
  # This checks rsync only when its section is enabled.
}

create_timeshift_snapshot() {
  # This function creates a Timeshift system snapshot.
  print_line
  # This prints a divider.
  printf '%s\n' "Timeshift snapshot section"
  # This prints the section title.
  printf '%s\n' "Purpose: rollback Linux system files after updates or config mistakes."
  # This explains why Timeshift exists.
  printf '%s\n' "Warning: snapshots on the same disk do not save you from disk death."
  # This explains the limitation.
  run_cmd sudo timeshift --create --comments "manual learning snapshot" --tags D
  # This asks Timeshift to create a daily-tagged snapshot with a comment.
}

build_restic_excludes() {
  # This function converts the EXCLUDES array into restic --exclude arguments.
  local args=()
  # This creates an empty array for exclude arguments.
  local excluded_path
  # This declares a variable used in the loop.
  for excluded_path in "${EXCLUDES[@]}"; do
    # This loops over every excluded folder.
    args+=(--exclude "$excluded_path")
    # This adds one restic exclude option for that folder.
  done
  # This ends the loop.
  printf '%s\0' "${args[@]}"
  # This prints the arguments separated by null characters.
}

run_restic_backup() {
  # This function runs a restic versioned backup.
  print_line
  # This prints a divider.
  printf '%s\n' "restic backup section"
  # This prints the section title.
  printf '%s\n' "Purpose: save file versions so corrupted files can be restored later."
  # This explains why restic exists.
  printf '%s\n' "Repository: $RESTIC_REPOSITORY"
  # This shows where restic will store backup data.
  local restic_exclude_args=()
  # This creates an empty array for restic excludes.
  while IFS= read -r -d '' item; do
    # This reads each null-separated exclude argument.
    restic_exclude_args+=("$item")
    # This appends that argument to the array.
  done < <(build_restic_excludes)
  # This feeds generated exclude arguments into the loop.
  run_cmd restic -r "$RESTIC_REPOSITORY" snapshots
  # This lists existing restic snapshots so you can see history.
  run_cmd restic -r "$RESTIC_REPOSITORY" backup "$BACKUP_SOURCE_HOME" "${restic_exclude_args[@]}"
  # This backs up your home folder while skipping excluded folders.
  run_cmd restic -r "$RESTIC_REPOSITORY" forget --prune --keep-hourly "$RESTIC_KEEP_HOURLY" --keep-daily "$RESTIC_KEEP_DAILY" --keep-weekly "$RESTIC_KEEP_WEEKLY"
  # This applies the retention policy and removes unneeded old backup chunks.
}

run_borg_backup() {
  # This function runs a Borg versioned backup.
  print_line
  # This prints a divider.
  printf '%s\n' "Borg backup section"
  # This prints the section title.
  printf '%s\n' "Purpose: Borg does the same kind of serious versioned backup as restic."
  # This explains why Borg exists.
  printf '%s\n' "Repository: $BORG_REPOSITORY"
  # This shows where Borg will store backup data.
  local archive_name
  # This declares the archive name variable.
  archive_name="home-{now:%Y-%m-%d-%H%M%S}"
  # This names each Borg backup with the current date and time.
  local borg_exclude_args=()
  # This creates an empty array for Borg excludes.
  local excluded_path
  # This declares the loop variable.
  for excluded_path in "${EXCLUDES[@]}"; do
    # This loops over excluded folders.
    borg_exclude_args+=(--exclude "$excluded_path")
    # This adds one Borg exclude option for that folder.
  done
  # This ends the exclude loop.
  run_cmd borg create --stats "$BORG_REPOSITORY::$archive_name" "$BACKUP_SOURCE_HOME" "${borg_exclude_args[@]}"
  # This creates a versioned Borg archive of your home folder.
  run_cmd borg prune --list "$BORG_REPOSITORY" --keep-hourly "$BORG_KEEP_HOURLY" --keep-daily "$BORG_KEEP_DAILY" --keep-weekly "$BORG_KEEP_WEEKLY"
  # This applies the Borg retention policy.
}

run_rsync_mirror() {
  # This function runs a simple rsync mirror.
  print_line
  # This prints a divider.
  printf '%s\n' "rsync mirror section"
  # This prints the section title.
  printf '%s\n' "Purpose: copy current files quickly to another folder or drive."
  # This explains why rsync exists.
  printf '%s\n' "Warning: --delete removes files from the mirror if they were deleted from source."
  # This warns about the dangerous part of mirrors.
  local rsync_exclude_args=()
  # This creates an empty array for rsync excludes.
  local excluded_path
  # This declares the loop variable.
  for excluded_path in "${EXCLUDES[@]}"; do
    # This loops over excluded folders.
    rsync_exclude_args+=(--exclude "$excluded_path")
    # This adds one rsync exclude option for that folder.
  done
  # This ends the exclude loop.
  run_cmd mkdir -p "$RSYNC_DESTINATION"
  # This creates the destination folder if it does not exist.
  run_cmd rsync -aHAX --numeric-ids --delete "${rsync_exclude_args[@]}" "$BACKUP_SOURCE_HOME/" "$RSYNC_DESTINATION/"
  # This mirrors your home folder to the destination folder.
}

show_scheduler_examples() {
  # This function prints examples for automating the script later.
  print_line
  # This prints a divider.
  printf '%s\n' "Automation examples"
  # This prints the section title.
  printf '%s\n' "cron example for daily 2:30 AM:"
  # This explains the cron example.
  printf '%s\n' "30 2 * * * /home/iscjmz/shopify/shopify/Pokemon/scripts/backup_snapshot_learning.sh >> /home/iscjmz/backup-learning.log 2>&1"
  # This prints a cron line without installing it.
  printf '%s\n' "systemd timers are also common in real Linux setups."
  # This mentions the other common scheduler.
}

show_restore_examples() {
  # This function prints restore commands you should learn before trusting backups.
  print_line
  # This prints a divider.
  printf '%s\n' "Restore practice examples"
  # This prints the section title.
  printf '%s\n' "restic list backups: restic -r \"$RESTIC_REPOSITORY\" snapshots"
  # This shows how to list restic snapshots.
  printf '%s\n' "restic restore one backup: restic -r \"$RESTIC_REPOSITORY\" restore latest --target /tmp/restore-test"
  # This shows how to restore restic files into a test folder.
  printf '%s\n' "Borg list backups: borg list \"$BORG_REPOSITORY\""
  # This shows how to list Borg archives.
  printf '%s\n' "Timeshift restore is usually easiest from the Timeshift app GUI."
  # This explains a common Timeshift restore path.
}

main() {
  # This is the main function that controls the script order.
  explain_plan
  # This prints the learning section first.
  check_tools
  # This checks whether enabled tools are installed.
  [[ "$ENABLE_TIMESHIFT" == "true" ]] && create_timeshift_snapshot
  # This runs the Timeshift section only when enabled.
  [[ "$ENABLE_RESTIC" == "true" ]] && run_restic_backup
  # This runs the restic section only when enabled.
  [[ "$ENABLE_BORG" == "true" ]] && run_borg_backup
  # This runs the Borg section only when enabled.
  [[ "$ENABLE_RSYNC_MIRROR" == "true" ]] && run_rsync_mirror
  # This runs the rsync mirror section only when enabled.
  show_scheduler_examples
  # This prints scheduling examples.
  show_restore_examples
  # This prints restore practice examples.
}

main "$@"
# This starts the script by calling the main function.
