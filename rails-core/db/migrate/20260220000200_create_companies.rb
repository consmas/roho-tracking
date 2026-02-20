class CreateCompanies < ActiveRecord::Migration[7.1]
  def change
    create_table :companies, id: :uuid do |t|
      t.string :name, null: false
      t.string :timezone, null: false, default: "UTC"
      t.timestamps
    end

    add_index :companies, :name
  end
end
