class CreateDevices < ActiveRecord::Migration[7.1]
  def change
    create_table :devices, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.references :vehicle, type: :uuid, foreign_key: true
      t.string :uid, null: false
      t.string :auth_token_digest, null: false
      t.integer :status, null: false, default: 0
      t.datetime :last_seen_at
      t.string :model
      t.timestamps
    end

    add_index :devices, :uid, unique: true
    add_index :devices, [:company_id, :status]
    add_index :devices, :last_seen_at
  end
end
