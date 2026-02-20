class CreateUsers < ActiveRecord::Migration[7.1]
  def change
    create_table :users, id: :uuid do |t|
      t.references :company, null: false, type: :uuid, foreign_key: true
      t.string :email, null: false
      t.string :password_digest, null: false
      t.integer :role, null: false, default: 0
      t.boolean :active, null: false, default: true
      t.timestamps
    end

    add_index :users, [:company_id, :email], unique: true
    add_index :users, :email, unique: true
  end
end
