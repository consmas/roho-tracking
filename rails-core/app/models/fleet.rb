class Fleet < ApplicationRecord
  belongs_to :company
  has_many :vehicles, dependent: :nullify

  validates :name, presence: true
end
