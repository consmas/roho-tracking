class AddCommandRefsToStreamSessions < ActiveRecord::Migration[7.1]
  def change
    add_column :stream_sessions, :start_command_id, :string
    add_column :stream_sessions, :stop_command_id, :string
    add_column :stream_sessions, :activation_deadline_at, :datetime
    add_column :stream_sessions, :ended_reason, :string

    add_index :stream_sessions, :start_command_id
    add_index :stream_sessions, :stop_command_id
    add_index :stream_sessions, :activation_deadline_at
    add_index :stream_sessions, [:device_id, :channel, :status]
  end
end
