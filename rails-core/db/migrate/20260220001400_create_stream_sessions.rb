class CreateStreamSessions < ActiveRecord::Migration[7.1]
  def change
    create_table :stream_sessions, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :device, null: false, type: :uuid, foreign_key: true
      t.references :user, null: false, type: :uuid, foreign_key: true
      t.string :session_id, null: false
      t.integer :channel, null: false, default: 1
      t.string :stream_name, null: false
      t.integer :status, null: false, default: 0
      t.jsonb :playback_urls, null: false, default: {}
      t.string :ingest_url
      t.datetime :started_at
      t.datetime :ended_at
      t.string :last_error
      t.jsonb :metadata, null: false, default: {}
      t.timestamps
    end

    add_index :stream_sessions, :session_id, unique: true
    add_index :stream_sessions, [:company_id, :created_at]
    add_index :stream_sessions, [:device_id, :status]
    add_index :stream_sessions, [:status, :created_at]
    add_index :stream_sessions, :playback_urls, using: :gin
    add_index :stream_sessions, :metadata, using: :gin
  end
end
